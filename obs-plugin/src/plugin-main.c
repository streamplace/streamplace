#include <string.h>

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
				    "\"stream_key_link\": \"https://stream.place/live\","
				    "\"servers\": [{\"name\": \"Local Streamplace node\", \"url\": \"http://127.0.0.1:38080\"}],"
				    "\"supported video codecs\": [\"h264\"],"
				    "\"supported audio codecs\": [\"aac\", \"opus\"],"
				    "\"recommended\": {\"keyint\": 1, \"bframes\": 0}"
				    "}";

/* Adds the entry to rtmp-services' services.json in the user's config
 * directory, which it reads in preference to its bundled copy. When OBS's
 * service updater replaces that file, the next startup adds the entry again. */
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

	obs_data_t *root = obs_data_create_from_json_file(config_file);
	if (!root && bundled_file)
		root = obs_data_create_from_json_file(bundled_file);

	obs_data_array_t *services = root ? obs_data_get_array(root, "services") : NULL;
	if (services) {
		obs_data_t *entry = obs_data_create_from_json(service_entry);
		const char *entry_json = obs_data_get_json(entry);
		bool current = false;

		for (size_t i = obs_data_array_count(services); i > 0; i--) {
			obs_data_t *service = obs_data_array_item(services, i - 1);
			if (strcmp(obs_data_get_string(service, "name"), "Streamplace") == 0) {
				if (strcmp(obs_data_get_json(service), entry_json) == 0)
					current = true;
				else
					obs_data_array_erase(services, i - 1);
			}
			obs_data_release(service);
		}

		if (!current) {
			obs_data_array_push_back(services, entry);
			os_mkdirs(config_dir);
			if (obs_data_save_json_pretty_safe(root, config_file, ".tmp", NULL))
				do_log(LOG_INFO, "Added the Streamplace service to %s", config_file);
			else
				do_log(LOG_WARNING, "Could not write %s", config_file);
		}

		obs_data_release(entry);
		obs_data_array_release(services);
	} else {
		do_log(LOG_WARNING, "Could not read rtmp-services' services.json");
	}

	obs_data_release(root);
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
