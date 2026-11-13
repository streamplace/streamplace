#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>

#include <jansson.h>
#include <obs-frontend-api.h>
#include <util/platform.h>

#include "http.h"
#include "streamplace.h"

OBS_DECLARE_MODULE()

MODULE_EXPORT const char *obs_module_description(void)
{
	return "Streams to a Streamplace node as fragmented MP4 over HTTP";
}

/* OBS's Settings → Stream page only knows the rtmp-services service types, so a
 * third-party output is reached through a services.json entry whose "protocol"
 * this plugin registers: rtmp-services hides the entry unless the protocol is
 * registered, and the frontend starts whichever output registered it. OBS
 * applies "recommended" to the stream encoders: a segment is one GoP, so 1s
 * keyframes keep latency low, and B-frames force viewers onto HLS. */
static const char service_entry[] = "{"
				    "\"name\": \"Streamplace\","
				    "\"common\": true,"
				    "\"protocol\": \"" STREAMPLACE_PROTOCOL "\","
				    "\"more_info_link\": \"https://stream.place/about\","
				    "\"stream_key_link\": \"streamplace-desktop:obs\","
				    "\"servers\": [{\"name\": \"Local Streamplace node\", \"url\": \"http://127.0.0.1:38080\"}],"
				    "\"supported video codecs\": [\"h264\", \"av1\"],"
				    "\"supported audio codecs\": [\"aac\", \"opus\"],"
				    "\"recommended\": {\"keyint\": 1, \"bframes\": 0}"
				    "}";

/* OBS paths are UTF-8, which jansson's fopen can't open on Windows. */
static json_t *load_json(const char *path)
{
	FILE *file = path ? os_fopen(path, "rb") : NULL;
	if (!file)
		return NULL;
	json_t *json = json_loadf(file, 0, NULL);
	fclose(file);
	return json;
}

/* Adds the entry to rtmp-services' services.json in the user's config
 * directory, which it reads in preference to its bundled copy. When OBS's
 * service updater replaces that file, the next startup adds the entry again.
 * The file is edited with jansson, as rtmp-services reads it: obs_data drops
 * arrays of strings (every service's codec lists), so it can't round-trip it. */
static void install_service_entry(void)
{
	obs_module_t *rtmp_services = obs_get_module("rtmp-services");
	if (!rtmp_services) {
		do_log(LOG_WARNING, "rtmp-services is not loaded; Streamplace will not be listed in Settings");
		return;
	}

	char *config_dir = obs_module_get_config_path(rtmp_services, "");
	char *config_file = obs_module_get_config_path(rtmp_services, "services.json");
	char *bundled_file = obs_find_module_file(rtmp_services, "services.json");

	json_t *root = load_json(config_file);
	if (!root)
		root = load_json(bundled_file);

	json_t *services = json_object_get(root, "services");
	if (json_is_array(services)) {
		json_t *entry = json_loads(service_entry, 0, NULL);
		bool current = false;

		for (size_t i = json_array_size(services); i > 0; i--) {
			json_t *service = json_array_get(services, i - 1);
			const char *name = json_string_value(json_object_get(service, "name"));
			if (name && strcmp(name, "Streamplace") == 0) {
				if (json_equal(service, entry))
					current = true;
				else
					json_array_remove(services, i - 1);
			}
		}

		if (!current) {
			json_array_append(services, entry);
			os_mkdirs(config_dir);
			char *json = json_dumps(root, JSON_INDENT(4));
			if (json && os_quick_write_utf8_file_safe(config_file, json, strlen(json), false, ".tmp", NULL))
				do_log(LOG_INFO, "Added the Streamplace service to %s", config_file);
			else
				do_log(LOG_WARNING, "Could not write %s", config_file);
			free(json);
		}

		json_decref(entry);
	} else {
		do_log(LOG_WARNING, "Could not read rtmp-services' services.json");
	}

	json_decref(root);
	bfree(bundled_file);
	bfree(config_file);
	bfree(config_dir);
}

static char *handoff_path;
static char *last_key;
static float handoff_elapsed;
static atomic_bool handoff_pending;
static atomic_bool handoff_stopped;

/* Settings can save an empty key after we connect. Refill it, but leave a
 * manually entered key alone; only rotate a key we previously applied. */
static void apply_handoff(void *unused)
{
	UNUSED_PARAMETER(unused);
	atomic_store(&handoff_pending, false);
	if (atomic_load(&handoff_stopped))
		return;

	json_t *handoff = load_json(handoff_path);
	const char *key = json_string_value(json_object_get(handoff, "key"));
	const char *did = json_string_value(json_object_get(handoff, "did"));
	if (!key || !*key || !did || !*did) {
		json_decref(handoff);
		return;
	}

	/* The frontend owns this service; only its settings need releasing. */
	obs_service_t *service = obs_frontend_get_streaming_service();
	if (service) {
		obs_data_t *settings = obs_service_get_settings(service);
		const char *name = obs_data_get_string(settings, "service");
		const char *current = obs_data_get_string(settings, "key");
		/* Recognize a handoff restored from the profile after restarting OBS. */
		if (strcmp(name, "Streamplace") == 0 && strcmp(current, key) == 0 &&
		    (!last_key || strcmp(last_key, key) != 0)) {
			free(last_key);
			last_key = strdup(key);
		}
		if (strcmp(name, "Streamplace") == 0 &&
		    (!*current || (last_key && strcmp(current, last_key) == 0)) &&
		    strcmp(current, key) != 0) {
			obs_data_set_string(settings, "key", key);
			obs_service_update(service, settings);
			obs_frontend_save_streaming_service();
			free(last_key);
			last_key = strdup(key);
			do_log(LOG_INFO, "Applied Desktop stream key for %s", did);
		}
		obs_data_release(settings);
	}
	json_decref(handoff);
}

/* The video tick only schedules work; file IO and service updates run on UI. */
static void handoff_tick(void *unused, float seconds)
{
	UNUSED_PARAMETER(unused);
	if (atomic_load(&handoff_stopped))
		return;
	handoff_elapsed += seconds;
	if (handoff_elapsed < 1.0f)
		return;
	handoff_elapsed = 0;
	if (!atomic_exchange(&handoff_pending, true))
		obs_queue_task(OBS_TASK_UI, apply_handoff, NULL, false);
}

static void handoff_event(enum obs_frontend_event event, void *unused)
{
	UNUSED_PARAMETER(unused);
	if (event == OBS_FRONTEND_EVENT_EXIT)
		atomic_store(&handoff_stopped, true);
}

bool obs_module_load(void)
{
	http_init();
	obs_register_output(&streamplace_output_info);
	do_log(LOG_INFO, "loaded version %s", PLUGIN_VERSION);
	return true;
}

void obs_module_post_load(void)
{
	install_service_entry();
	handoff_path = obs_module_config_path("stream-key.json");
	obs_frontend_add_event_callback(handoff_event, NULL);
	obs_add_tick_callback(handoff_tick, NULL);
}

void obs_module_unload(void)
{
	atomic_store(&handoff_stopped, true);
	obs_remove_tick_callback(handoff_tick, NULL);
	obs_frontend_remove_event_callback(handoff_event, NULL);
	bfree(handoff_path);
	free(last_key);
	http_free();
}
