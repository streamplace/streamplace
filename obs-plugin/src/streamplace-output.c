#include <inttypes.h>
#include <pthread.h>
#include <string.h>

#include <obs-avc.h>
#include <util/array-serializer.h>
#include <util/platform.h>
#include <util/threading.h>

#include "fmp4.h"
#include "http.h"
#include "streamplace.h"

/* Same request and response shapes as OBS's multitrack video ("GoLiveApi")
 * configuration endpoint, so one server endpoint can configure both this
 * plugin and OBS's built-in Enhanced RTMP multitrack output. */
#define CONFIG_PATH "/api/ingest/client-configuration"
#define CONFIG_SCHEMA_VERSION "2025-01-25"
#define INGEST_PROTOCOL "FMP4"
#define MAX_QUEUED_BYTES (64 * 1024 * 1024)

struct sp_output {
	obs_output_t *output;
	obs_encoder_group_t *encoder_group;

	pthread_t connect_thread;
	bool connect_thread_active;
	pthread_t send_thread;
	bool send_thread_active;
	struct http_conn conn;

	pthread_mutex_t mutex;
	pthread_cond_t cond;
	/* Everything below is guarded by mutex. */
	bool stopping;  /* stop() was called */
	bool capturing; /* packets are muxed and queued for the send thread */
	bool finishing; /* the send thread should wrap up and exit */
	int error_code; /* why it is finishing, or OBS_OUTPUT_SUCCESS */
	bool mux_started;
	size_t video_tracks, audio_tracks;
	struct fmp4_mux mux;
	struct serializer queue_s;
	struct array_output_data queue;
	uint64_t total_bytes;
};

static bool is_stopping(struct sp_output *sp)
{
	pthread_mutex_lock(&sp->mutex);
	bool stopping = sp->stopping;
	pthread_mutex_unlock(&sp->mutex);
	return stopping;
}

/* ---------------------------------------------------------------------------
 * Stream configuration */

static obs_data_t *build_config_request(const char *key)
{
	obs_data_t *post = obs_data_create();
	obs_data_set_string(post, "service", "Streamplace");
	obs_data_set_string(post, "schema_version", CONFIG_SCHEMA_VERSION);
	obs_data_set_string(post, "authentication", key);

	obs_data_t *client = obs_data_create();
	obs_data_set_string(client, "name", "obs-studio");
	obs_data_set_string(client, "version", obs_get_version_string());
	obs_data_set_string(client, "plugin_version", PLUGIN_VERSION);
	obs_data_set_string(client, "supported_codecs", "h264");
	obs_data_set_obj(post, "client", client);
	obs_data_release(client);

	obs_data_t *preferences = obs_data_create();
	obs_data_set_bool(preferences, "vod_track_audio", false);
	struct obs_video_info ovi;
	if (obs_get_video_info(&ovi)) {
		obs_data_set_int(preferences, "composition_gpu_index", ovi.adapter);
		obs_data_t *canvas = obs_data_create();
		obs_data_set_int(canvas, "width", ovi.output_width);
		obs_data_set_int(canvas, "height", ovi.output_height);
		obs_data_set_int(canvas, "canvas_width", ovi.base_width);
		obs_data_set_int(canvas, "canvas_height", ovi.base_height);
		obs_data_t *framerate = obs_data_create();
		obs_data_set_int(framerate, "numerator", ovi.fps_num);
		obs_data_set_int(framerate, "denominator", ovi.fps_den);
		obs_data_set_obj(canvas, "framerate", framerate);
		obs_data_release(framerate);
		obs_data_array_t *canvases = obs_data_array_create();
		obs_data_array_push_back(canvases, canvas);
		obs_data_release(canvas);
		obs_data_set_array(preferences, "canvases", canvases);
		obs_data_array_release(canvases);
	}
	struct obs_audio_info oai;
	if (obs_get_audio_info(&oai)) {
		obs_data_set_int(preferences, "audio_samples_per_sec", oai.samples_per_sec);
		obs_data_set_int(preferences, "audio_channels", get_audio_channels(oai.speakers));
	}
	obs_data_set_obj(post, "preferences", preferences);
	obs_data_release(preferences);

	return post;
}

/* obs_data has no arrays of strings, so the codec list is patched in as JSON. */
static char *config_request_json(const char *key)
{
	obs_data_t *post = build_config_request(key);
	struct dstr json = {0};
	dstr_copy(&json, obs_data_get_json(post));
	dstr_replace(&json, "\"supported_codecs\":\"h264\"", "\"supported_codecs\":[\"h264\"]");
	obs_data_release(post);
	return json.array;
}

static obs_data_t *fetch_config(const char *server, const char *key, struct dstr *err)
{
	struct dstr url = {0};
	dstr_copy(&url, server);
	while (url.len && url.array[url.len - 1] == '/')
		dstr_resize(&url, url.len - 1);
	dstr_cat(&url, CONFIG_PATH);

	char *body = config_request_json(key);
	struct dstr response = {0};
	long status = 0;
	obs_data_t *config = NULL;

	do_log(LOG_INFO, "Requesting stream configuration from %s", url.array);
	if (http_post_json(url.array, body, &status, &response, err)) {
		if (status != 200) {
			dstr_printf(err, "Stream configuration request to %s failed with HTTP %ld: %s", url.array, status,
				    response.array ? response.array : "");
		} else if (!(config = obs_data_create_from_json(response.array))) {
			dstr_printf(err, "Stream configuration from %s is not valid JSON", url.array);
		} else {
			do_log(LOG_INFO, "Stream configuration: %s", response.array);
		}
	}

	if (config) {
		obs_data_t *result = obs_data_get_obj(config, "status");
		const char *outcome = obs_data_get_string(result, "result");
		const char *message = obs_data_get_string(result, "html_en_us");
		if (strcmp(outcome, "error") == 0) {
			dstr_copy(err, *message ? message : "The server refused to configure the stream");
			obs_data_release(config);
			config = NULL;
		} else if (strcmp(outcome, "warning") == 0) {
			do_log(LOG_WARNING, "Stream configuration warning: %s", message);
		}
		obs_data_release(result);
	}

	bfree(body);
	dstr_free(&response);
	dstr_free(&url);
	return config;
}

static bool find_ingest_url(obs_data_t *config, const char *key, struct dstr *url, struct dstr *err)
{
	obs_data_array_t *endpoints = obs_data_get_array(config, "ingest_endpoints");
	for (size_t i = 0; i < obs_data_array_count(endpoints) && dstr_is_empty(url); i++) {
		obs_data_t *endpoint = obs_data_array_item(endpoints, i);
		if (astrcmpi(obs_data_get_string(endpoint, "protocol"), INGEST_PROTOCOL) == 0) {
			dstr_copy(url, obs_data_get_string(endpoint, "url_template"));
			dstr_replace(url, "{stream_key}", key);
		}
		obs_data_release(endpoint);
	}
	obs_data_array_release(endpoints);

	if (dstr_is_empty(url))
		dstr_printf(err, "The server offered no %s ingest endpoint", INGEST_PROTOCOL);
	return !dstr_is_empty(url);
}

static bool encoder_type_available(const char *type)
{
	const char *id;
	for (size_t i = 0; obs_enum_encoder_types(i, &id); i++) {
		if (strcmp(id, type) == 0)
			return true;
	}
	return false;
}

static const char *audio_encoder_for_codec(const char *codec)
{
	const char *id;
	for (size_t i = 0; obs_enum_encoder_types(i, &id); i++) {
		if (obs_get_encoder_type(id) == OBS_ENCODER_AUDIO && strcmp(obs_get_encoder_codec(id), codec) == 0 &&
		    !(obs_get_encoder_caps(id) & (OBS_ENCODER_CAP_DEPRECATED | OBS_ENCODER_CAP_INTERNAL)))
			return id;
	}
	return NULL;
}

struct enum_name {
	const char *name;
	int value;
};

static const struct enum_name scale_types[] = {
	{"OBS_SCALE_POINT", OBS_SCALE_POINT},       {"OBS_SCALE_BICUBIC", OBS_SCALE_BICUBIC},
	{"OBS_SCALE_BILINEAR", OBS_SCALE_BILINEAR}, {"OBS_SCALE_LANCZOS", OBS_SCALE_LANCZOS},
	{"OBS_SCALE_AREA", OBS_SCALE_AREA},         {NULL, 0},
};

static const struct enum_name video_formats[] = {
	{"VIDEO_FORMAT_NV12", VIDEO_FORMAT_NV12}, {"VIDEO_FORMAT_I420", VIDEO_FORMAT_I420},
	{"VIDEO_FORMAT_I444", VIDEO_FORMAT_I444}, {"VIDEO_FORMAT_I010", VIDEO_FORMAT_I010},
	{"VIDEO_FORMAT_P010", VIDEO_FORMAT_P010}, {NULL, 0},
};

/* Unset fields get the same defaults OBS's multitrack output uses. */
static int lookup_enum(const struct enum_name *names, const char *name, int fallback)
{
	for (; names->name; names++) {
		if (strcmp(names->name, name) == 0)
			return names->value;
	}
	return fallback;
}

static obs_encoder_t *create_video_encoder(obs_data_t *config, size_t idx, const struct obs_video_info *ovi,
					   struct dstr *err)
{
	const char *type = obs_data_get_string(config, "type");
	if (!encoder_type_available(type)) {
		dstr_printf(err, "Video encoder '%s' is not available in this OBS", type);
		return NULL;
	}
	if (strcmp(obs_get_encoder_codec(type), "h264") != 0) {
		dstr_printf(err, "Video encoder '%s' is not H.264, the only video codec Streamplace accepts", type);
		return NULL;
	}

	obs_data_t *settings = obs_data_get_obj(config, "settings");
	if (!settings)
		settings = obs_data_create();
	/* Keeps keyframes aligned across renditions. */
	obs_data_set_bool(settings, "disable_scenecut", true);

	struct dstr name = {0};
	dstr_printf(&name, "streamplace video %d", (int)idx);
	obs_encoder_t *encoder = obs_video_encoder_create(type, name.array, settings, NULL);
	obs_data_release(settings);
	dstr_free(&name);
	if (!encoder) {
		dstr_printf(err, "Could not create video encoder '%s'", type);
		return NULL;
	}

	obs_encoder_set_video(encoder, obs_get_video());
	uint32_t width = (uint32_t)obs_data_get_int(config, "width");
	uint32_t height = (uint32_t)obs_data_get_int(config, "height");
	if (width && height)
		obs_encoder_set_scaled_size(encoder, width, height);
	obs_encoder_set_gpu_scale_type(encoder, lookup_enum(scale_types, obs_data_get_string(config, "gpu_scale_type"),
							    OBS_SCALE_BICUBIC));
	obs_encoder_set_preferred_video_format(
		encoder, lookup_enum(video_formats, obs_data_get_string(config, "format"), VIDEO_FORMAT_NV12));

	/* Bounded to 32 bits so the products below cannot overflow to zero. */
	obs_data_t *framerate = obs_data_get_obj(config, "framerate");
	long long num = obs_data_get_int(framerate, "numerator");
	long long den = obs_data_get_int(framerate, "denominator");
	if (num > 0 && den > 0 && num <= UINT32_MAX && den <= UINT32_MAX) {
		uint64_t divisor = ((uint64_t)ovi->fps_num * (uint64_t)den) / ((uint64_t)num * ovi->fps_den);
		if (divisor > 1)
			obs_encoder_set_frame_rate_divisor(encoder, (uint32_t)divisor);
	}
	obs_data_release(framerate);

	return encoder;
}

static bool create_video_encoders(struct sp_output *sp, obs_data_array_t *configs, struct dstr *err)
{
	size_t count = obs_data_array_count(configs);
	if (count > MAX_OUTPUT_VIDEO_ENCODERS) {
		dstr_printf(err, "The server asked for %d video tracks; at most %d are supported", (int)count,
			    MAX_OUTPUT_VIDEO_ENCODERS);
		return false;
	}

	struct obs_video_info ovi;
	obs_get_video_info(&ovi);

	/* Grouped encoders start together, so their keyframes line up. */
	obs_encoder_group_t *group = obs_encoder_group_create();
	obs_encoder_t *encoders[MAX_OUTPUT_VIDEO_ENCODERS] = {0};
	bool ok = true;
	for (size_t i = 0; ok && i < count; i++) {
		obs_data_t *config = obs_data_array_item(configs, i);
		encoders[i] = create_video_encoder(config, i, &ovi, err);
		ok = encoders[i] && obs_encoder_set_group(encoders[i], group);
		obs_data_release(config);
	}

	if (ok) {
		for (size_t i = 0; i < MAX_OUTPUT_VIDEO_ENCODERS; i++)
			obs_output_set_video_encoder2(sp->output, encoders[i], i);
		obs_encoder_group_destroy(sp->encoder_group);
		sp->encoder_group = group;
	} else {
		obs_encoder_group_destroy(group);
	}
	for (size_t i = 0; i < count; i++)
		obs_encoder_release(encoders[i]);
	return ok;
}

/* track_id is the zero-based OBS audio mixer index (the mixer's "Track 1" is 0). */
static bool create_audio_encoders(struct sp_output *sp, obs_data_array_t *configs, struct dstr *err)
{
	size_t count = obs_data_array_count(configs);
	if (count > MAX_OUTPUT_AUDIO_ENCODERS) {
		dstr_printf(err, "The server asked for %d audio tracks; at most %d are supported", (int)count,
			    MAX_OUTPUT_AUDIO_ENCODERS);
		return false;
	}

	obs_encoder_t *encoders[MAX_OUTPUT_AUDIO_ENCODERS] = {0};
	bool ok = true;
	for (size_t i = 0; ok && i < count; i++) {
		obs_data_t *config = obs_data_array_item(configs, i);
		const char *codec = obs_data_get_string(config, "codec");
		const char *type = audio_encoder_for_codec(*codec ? codec : "aac");
		int mixer = (int)obs_data_get_int(config, "track_id");

		if (!type) {
			dstr_printf(err, "No audio encoder for '%s' is available in this OBS", codec);
			ok = false;
		} else if (mixer < 0 || mixer >= MAX_AUDIO_MIXES) {
			dstr_printf(err, "Audio track_id %d is not an OBS mixer track (0-%d)", mixer, MAX_AUDIO_MIXES - 1);
			ok = false;
		} else {
			obs_data_t *settings = obs_data_get_obj(config, "settings");
			struct dstr name = {0};
			dstr_printf(&name, "streamplace audio %d", (int)i);
			encoders[i] = obs_audio_encoder_create(type, name.array, settings, (size_t)mixer, NULL);
			if (encoders[i])
				obs_encoder_set_audio(encoders[i], obs_get_audio());
			else
				dstr_printf(err, "Could not create audio encoder '%s'", type);
			ok = encoders[i] != NULL;
			obs_data_release(settings);
			dstr_free(&name);
		}
		obs_data_release(config);
	}

	if (ok) {
		for (size_t i = 0; i < MAX_OUTPUT_AUDIO_ENCODERS; i++)
			obs_output_set_audio_encoder(sp->output, encoders[i], i);
	}
	for (size_t i = 0; i < count; i++)
		obs_encoder_release(encoders[i]);
	return ok;
}

/* Without encoder configurations, the encoders from OBS's output settings stay. */
static bool apply_encoder_configs(struct sp_output *sp, obs_data_t *config, struct dstr *err)
{
	obs_data_array_t *video = obs_data_get_array(config, "encoder_configurations");
	obs_data_t *audio_configs = obs_data_get_obj(config, "audio_configurations");
	obs_data_array_t *audio = obs_data_get_array(audio_configs, "live");

	bool ok = (!obs_data_array_count(video) || create_video_encoders(sp, video, err)) &&
		  (!obs_data_array_count(audio) || create_audio_encoders(sp, audio, err));

	obs_data_array_release(audio);
	obs_data_release(audio_configs);
	obs_data_array_release(video);
	return ok;
}

/* ---------------------------------------------------------------------------
 * Muxing, on OBS's output thread */

static bool add_video_track(struct sp_output *sp, obs_encoder_t *encoder, struct dstr *err)
{
	uint8_t *extra;
	size_t extra_size;
	if (strcmp(obs_encoder_get_codec(encoder), "h264") != 0) {
		dstr_printf(err, "Streamplace only accepts H.264 video, not %s", obs_encoder_get_codec(encoder));
		return false;
	}
	if (!obs_encoder_get_extra_data(encoder, &extra, &extra_size)) {
		dstr_printf(err, "Video encoder '%s' has no codec headers", obs_encoder_get_name(encoder));
		return false;
	}

	const struct video_output_info *voi = video_output_get_info(obs_encoder_video(encoder));
	struct fmp4_track track = {
		.codec = FMP4_H264,
		.timescale = voi->fps_num,
		.timebase_num = voi->fps_den * obs_encoder_get_frame_rate_divisor(encoder),
		.width = obs_encoder_get_width(encoder),
		.height = obs_encoder_get_height(encoder),
	};
	track.config_size = obs_parse_avc_header(&track.config, extra, extra_size);
	fmp4_add_track(&sp->mux, &track);
	bfree(track.config);
	return true;
}

static bool add_audio_track(struct sp_output *sp, obs_encoder_t *encoder, struct dstr *err)
{
	const char *codec = obs_encoder_get_codec(encoder);
	uint8_t *extra = NULL;
	size_t extra_size = 0;
	obs_encoder_get_extra_data(encoder, &extra, &extra_size);

	struct fmp4_track track = {
		.timescale = (uint32_t)audio_output_get_sample_rate(obs_encoder_audio(encoder)),
		.timebase_num = 1,
		.channels = (uint32_t)audio_output_get_channels(obs_encoder_audio(encoder)),
		.sample_rate = obs_encoder_get_sample_rate(encoder),
		.config = extra,
		.config_size = extra_size,
	};
	if (strcmp(codec, "aac") == 0 && extra_size) {
		track.codec = FMP4_AAC;
		obs_data_t *settings = obs_encoder_get_settings(encoder);
		track.bitrate = (uint32_t)obs_data_get_int(settings, "bitrate") * 1000;
		obs_data_release(settings);
	} else if (strcmp(codec, "opus") == 0 && extra_size >= 19 && memcmp(extra, "OpusHead", 8) == 0) {
		track.codec = FMP4_OPUS;
		track.sample_rate = 48000;
	} else {
		dstr_printf(err, "Audio encoder '%s' (%s) has no usable codec headers", obs_encoder_get_name(encoder),
			    codec);
		return false;
	}

	fmp4_add_track(&sp->mux, &track);
	return true;
}

static bool start_mux(struct sp_output *sp, struct dstr *err)
{
	obs_encoder_t *encoder;
	while (sp->video_tracks < MAX_OUTPUT_VIDEO_ENCODERS &&
	       (encoder = obs_output_get_video_encoder2(sp->output, sp->video_tracks))) {
		if (!add_video_track(sp, encoder, err))
			return false;
		sp->video_tracks++;
	}
	while (sp->audio_tracks < MAX_OUTPUT_AUDIO_ENCODERS &&
	       (encoder = obs_output_get_audio_encoder(sp->output, sp->audio_tracks))) {
		if (!add_audio_track(sp, encoder, err))
			return false;
		sp->audio_tracks++;
	}

	do_log(LOG_INFO, "Muxing %d video and %d audio tracks", (int)sp->video_tracks, (int)sp->audio_tracks);
	sp->mux_started = true;
	return true;
}

static void finish_with_error(struct sp_output *sp, int code)
{
	sp->finishing = true;
	sp->error_code = code;
	pthread_cond_signal(&sp->cond);
}

static void sp_encoded_packet(void *data, struct encoder_packet *packet)
{
	struct sp_output *sp = data;
	pthread_mutex_lock(&sp->mutex);
	if (!sp->capturing || sp->finishing) {
		pthread_mutex_unlock(&sp->mutex);
		return;
	}

	if (!packet) {
		do_log(LOG_ERROR, "An encoder failed");
		finish_with_error(sp, OBS_OUTPUT_ENCODE_ERROR);
		pthread_mutex_unlock(&sp->mutex);
		return;
	}

	if (!sp->mux_started) {
		struct dstr err = {0};
		if (!start_mux(sp, &err)) {
			do_log(LOG_ERROR, "%s", err.array);
			obs_output_set_last_error(sp->output, err.array);
			finish_with_error(sp, OBS_OUTPUT_UNSUPPORTED);
		}
		dstr_free(&err);
	}

	bool video = packet->type == OBS_ENCODER_VIDEO;
	size_t track_count = video ? sp->video_tracks : sp->audio_tracks;
	if (sp->mux_started && packet->track_idx < track_count) {
		struct encoder_packet owned;
		if (video)
			obs_parse_avc_packet(&owned, packet);
		else
			obs_encoder_packet_ref(&owned, packet);

		size_t track = video ? packet->track_idx : sp->video_tracks + packet->track_idx;
		fmp4_push(&sp->mux, track, &owned);
		/* A fragment per sample on the first track keeps latency to one frame. */
		if (track == 0)
			fmp4_write_fragment(&sp->mux, &sp->queue_s);

		if (sp->queue.bytes.num + sp->mux.held_bytes > MAX_QUEUED_BYTES) {
			do_log(LOG_ERROR, "Over %d MiB of stream data are waiting to be sent",
			       MAX_QUEUED_BYTES / (1024 * 1024));
			finish_with_error(sp, OBS_OUTPUT_DISCONNECTED);
		}
		pthread_cond_signal(&sp->cond);
	}
	pthread_mutex_unlock(&sp->mutex);
}

/* ---------------------------------------------------------------------------
 * Threads */

static void *send_thread(void *data)
{
	struct sp_output *sp = data;
	os_set_thread_name("streamplace-send");

	int code = OBS_OUTPUT_SUCCESS;
	bool finished = false;
	while (!finished) {
		pthread_mutex_lock(&sp->mutex);
		while (!sp->queue.bytes.num && !sp->finishing)
			pthread_cond_wait(&sp->cond, &sp->mutex);
		if (sp->finishing) {
			finished = true;
			code = sp->error_code;
			sp->capturing = false;
			if (code == OBS_OUTPUT_SUCCESS && sp->mux_started)
				fmp4_finish(&sp->mux, &sp->queue_s);
		}
		struct darray bytes = sp->queue.bytes.da;
		memset(&sp->queue, 0, sizeof(sp->queue));
		pthread_mutex_unlock(&sp->mutex);

		if (bytes.num && code == OBS_OUTPUT_SUCCESS) {
			if (http_write_chunk(&sp->conn, bytes.array, bytes.num)) {
				pthread_mutex_lock(&sp->mutex);
				sp->total_bytes += bytes.num;
				pthread_mutex_unlock(&sp->mutex);
			} else {
				do_log(LOG_WARNING, "Lost the connection to the server");
				code = OBS_OUTPUT_DISCONNECTED;
				pthread_mutex_lock(&sp->mutex);
				sp->capturing = false;
				pthread_mutex_unlock(&sp->mutex);
				finished = true;
			}
		}
		darray_free(&bytes);
	}

	if (code == OBS_OUTPUT_SUCCESS)
		do_log(LOG_INFO, "Stream ended; server responded with HTTP %ld", http_end_chunked_post(&sp->conn));
	http_close(&sp->conn);

	if (code == OBS_OUTPUT_SUCCESS || is_stopping(sp))
		obs_output_end_data_capture(sp->output);
	else
		obs_output_signal_stop(sp->output, code);
	return NULL;
}

static bool start_streaming(struct sp_output *sp, const char *url, struct dstr *err)
{
	if (!http_begin_chunked_post(&sp->conn, url, "video/mp4", err))
		return false;

	if (!obs_output_can_begin_data_capture(sp->output, 0) || !obs_output_initialize_encoders(sp->output, 0)) {
		dstr_copy(err, "Could not initialize the encoders");
		http_close(&sp->conn);
		return false;
	}

	pthread_mutex_lock(&sp->mutex);
	if (!sp->stopping)
		sp->send_thread_active = pthread_create(&sp->send_thread, NULL, send_thread, sp) == 0;
	sp->capturing = sp->send_thread_active;
	pthread_mutex_unlock(&sp->mutex);
	if (!sp->send_thread_active) {
		http_close(&sp->conn);
		return false;
	}

	do_log(LOG_INFO, "Streaming to %s", url);
	if (!obs_output_begin_data_capture(sp->output, 0)) {
		/* The send thread owns the connection now; it ends the stream. */
		pthread_mutex_lock(&sp->mutex);
		finish_with_error(sp, OBS_OUTPUT_ERROR);
		pthread_mutex_unlock(&sp->mutex);
	}
	return true;
}

static bool connect_and_start(struct sp_output *sp, struct dstr *err)
{
	obs_service_t *service = obs_output_get_service(sp->output);
	const char *server = obs_service_get_connect_info(service, OBS_SERVICE_CONNECT_INFO_SERVER_URL);
	const char *key = obs_service_get_connect_info(service, OBS_SERVICE_CONNECT_INFO_STREAM_ID);
	if (!server || !*server) {
		dstr_copy(err, "No Streamplace server is configured");
		return false;
	}

	obs_data_t *config = fetch_config(server, key ? key : "", err);
	if (!config)
		return false;

	struct dstr url = {0};
	bool ok = find_ingest_url(config, key ? key : "", &url, err) && !is_stopping(sp) &&
		  apply_encoder_configs(sp, config, err) && !is_stopping(sp) && start_streaming(sp, url.array, err);
	obs_data_release(config);
	dstr_free(&url);
	return ok;
}

static void *connect_thread(void *data)
{
	struct sp_output *sp = data;
	os_set_thread_name("streamplace-connect");

	struct dstr err = {0};
	if (!connect_and_start(sp, &err) && !is_stopping(sp)) {
		do_log(LOG_WARNING, "Could not start streaming: %s", err.array);
		obs_output_set_last_error(sp->output, err.array);
		obs_output_signal_stop(sp->output, OBS_OUTPUT_CONNECT_FAILED);
	}
	dstr_free(&err);
	return NULL;
}

static void join_threads(struct sp_output *sp)
{
	if (sp->connect_thread_active) {
		pthread_join(sp->connect_thread, NULL);
		sp->connect_thread_active = false;
	}
	if (sp->send_thread_active) {
		pthread_join(sp->send_thread, NULL);
		sp->send_thread_active = false;
	}
}

/* ---------------------------------------------------------------------------
 * obs_output_info */

static const char *sp_get_name(void *type_data)
{
	UNUSED_PARAMETER(type_data);
	return "Streamplace";
}

static void *sp_create(obs_data_t *settings, obs_output_t *output)
{
	UNUSED_PARAMETER(settings);
	struct sp_output *sp = bzalloc(sizeof(*sp));
	sp->output = output;
	pthread_mutex_init(&sp->mutex, NULL);
	pthread_cond_init(&sp->cond, NULL);
	array_output_serializer_init(&sp->queue_s, &sp->queue);
	return sp;
}

static void sp_destroy(void *data)
{
	struct sp_output *sp = data;
	join_threads(sp);
	fmp4_free(&sp->mux);
	array_output_serializer_free(&sp->queue);
	obs_encoder_group_destroy(sp->encoder_group);
	pthread_cond_destroy(&sp->cond);
	pthread_mutex_destroy(&sp->mutex);
	bfree(sp);
}

static bool sp_start(void *data)
{
	struct sp_output *sp = data;
	if (!obs_output_can_begin_data_capture(sp->output, 0))
		return false;

	/* Threads from a previous run have already wound down by now. */
	join_threads(sp);
	fmp4_free(&sp->mux);
	array_output_serializer_free(&sp->queue);
	array_output_serializer_init(&sp->queue_s, &sp->queue);
	sp->stopping = false;
	sp->capturing = false;
	sp->finishing = false;
	sp->error_code = OBS_OUTPUT_SUCCESS;
	sp->mux_started = false;
	sp->video_tracks = 0;
	sp->audio_tracks = 0;
	sp->total_bytes = 0;

	sp->connect_thread_active = pthread_create(&sp->connect_thread, NULL, connect_thread, sp) == 0;
	return sp->connect_thread_active;
}

static void sp_stop(void *data, uint64_t ts)
{
	UNUSED_PARAMETER(ts);
	struct sp_output *sp = data;

	pthread_mutex_lock(&sp->mutex);
	sp->stopping = true;
	pthread_mutex_unlock(&sp->mutex);

	if (sp->connect_thread_active) {
		pthread_join(sp->connect_thread, NULL);
		sp->connect_thread_active = false;
	}

	pthread_mutex_lock(&sp->mutex);
	if (sp->capturing && !sp->finishing)
		finish_with_error(sp, OBS_OUTPUT_SUCCESS);
	pthread_mutex_unlock(&sp->mutex);

	/* With no send thread, nothing else will report the stop. */
	if (!sp->send_thread_active)
		obs_output_signal_stop(sp->output, OBS_OUTPUT_SUCCESS);
}

static uint64_t sp_get_total_bytes(void *data)
{
	struct sp_output *sp = data;
	pthread_mutex_lock(&sp->mutex);
	uint64_t total = sp->total_bytes;
	pthread_mutex_unlock(&sp->mutex);
	return total;
}

struct obs_output_info streamplace_output_info = {
	.id = "streamplace_output",
	.flags = OBS_OUTPUT_AV | OBS_OUTPUT_ENCODED | OBS_OUTPUT_SERVICE | OBS_OUTPUT_MULTI_TRACK_AV,
	.protocols = STREAMPLACE_PROTOCOL,
	.encoded_video_codecs = "h264",
	.encoded_audio_codecs = "aac;opus",
	.get_name = sp_get_name,
	.create = sp_create,
	.destroy = sp_destroy,
	.start = sp_start,
	.stop = sp_stop,
	.encoded_packet = sp_encoded_packet,
	.get_total_bytes = sp_get_total_bytes,
};
