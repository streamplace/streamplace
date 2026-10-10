#include <stdlib.h>
#include <string.h>

#include <jansson.h>
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
 * registered, and the frontend starts whichever output registered it. */
static const char service_entry[] = "{"
				    "\"name\": \"Streamplace\","
				    "\"common\": true,"
				    "\"protocol\": \"" STREAMPLACE_PROTOCOL "\","
				    "\"more_info_link\": \"https://stream.place/about\","
				    "\"stream_key_link\": \"https://stream.place/live\","
				    "\"servers\": [{\"name\": \"Local Streamplace node\", \"url\": \"http://127.0.0.1:38080\"}],"
				    "\"supported video codecs\": [\"h264\"],"
				    "\"supported audio codecs\": [\"aac\", \"opus\"]"
				    "}";

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

	json_t *root = json_load_file(config_file, 0, NULL);
	if (!root && bundled_file)
		root = json_load_file(bundled_file, 0, NULL);

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
}

void obs_module_unload(void)
{
	http_free();
}
