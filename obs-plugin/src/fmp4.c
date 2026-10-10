#include "fmp4.h"

#include <string.h>

#include <util/bmem.h>

#define TFHD_DEFAULT_BASE_IS_MOOF 0x020000
/* data-offset, sample-duration, sample-size, sample-flags, composition-time-offset */
#define TRUN_FLAGS 0x000F01
#define SAMPLE_FLAGS_SYNC 0x02000000
#define SAMPLE_FLAGS_NON_SYNC 0x01010000

static const uint32_t unity_matrix[9] = {0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000};

static int64_t box_begin(struct serializer *s, const char *type)
{
	int64_t start = serializer_get_pos(s);
	s_wb32(s, 0);
	s_write(s, type, 4);
	return start;
}

static int64_t fullbox_begin(struct serializer *s, const char *type, uint8_t version, uint32_t flags)
{
	int64_t start = box_begin(s, type);
	s_w8(s, version);
	s_wb24(s, flags);
	return start;
}

static void patch_u32(struct serializer *s, int64_t pos, uint32_t value)
{
	int64_t end = serializer_get_pos(s);
	serializer_seek(s, pos, SERIALIZE_SEEK_START);
	s_wb32(s, value);
	serializer_seek(s, end, SERIALIZE_SEEK_START);
}

static void box_end(struct serializer *s, int64_t start)
{
	patch_u32(s, start, (uint32_t)(serializer_get_pos(s) - start));
}

static void write_zeros(struct serializer *s, size_t count)
{
	while (count--)
		s_w8(s, 0);
}

static void write_matrix(struct serializer *s)
{
	for (size_t i = 0; i < 9; i++)
		s_wb32(s, unity_matrix[i]);
}

static bool is_video(const struct fmp4_track *track)
{
	return track->codec == FMP4_H264;
}

void fmp4_free(struct fmp4_mux *mux)
{
	for (size_t i = 0; i < mux->tracks.num; i++) {
		struct fmp4_track *track = &mux->tracks.array[i];
		if (track->has_pending)
			obs_encoder_packet_release(&track->pending);
		for (size_t j = 0; j < track->samples.num; j++)
			obs_encoder_packet_release(&track->samples.array[j].packet);
		da_free(track->samples);
		bfree(track->config);
	}
	da_free(mux->tracks);
	mux->sequence = 0;
	mux->init_written = false;
}

size_t fmp4_add_track(struct fmp4_mux *mux, const struct fmp4_track *info)
{
	struct fmp4_track *track = da_push_back_new(mux->tracks);
	track->id = (uint32_t)mux->tracks.num;
	track->codec = info->codec;
	track->timescale = info->timescale;
	track->timebase_num = info->timebase_num;
	track->width = info->width;
	track->height = info->height;
	track->channels = info->channels;
	track->sample_rate = info->sample_rate;
	track->bitrate = info->bitrate;
	track->config = bmemdup(info->config, info->config_size);
	track->config_size = info->config_size;
	return mux->tracks.num - 1;
}

static void write_mvhd(struct serializer *s, uint32_t next_track_id)
{
	int64_t start = fullbox_begin(s, "mvhd", 0, 0);
	s_wb32(s, 0);          // creation_time
	s_wb32(s, 0);          // modification_time
	s_wb32(s, 1000);       // timescale
	s_wb32(s, 0);          // duration
	s_wb32(s, 0x00010000); // rate
	s_wb16(s, 0x0100);     // volume
	write_zeros(s, 10);    // reserved
	write_matrix(s);
	write_zeros(s, 24); // pre_defined
	s_wb32(s, next_track_id);
	box_end(s, start);
}

static void write_tkhd(struct serializer *s, const struct fmp4_track *track)
{
	int64_t start = fullbox_begin(s, "tkhd", 0, 0x000003); // enabled, in movie
	s_wb32(s, 0);                                           // creation_time
	s_wb32(s, 0);                                           // modification_time
	s_wb32(s, track->id);
	s_wb32(s, 0);                              // reserved
	s_wb32(s, 0);                              // duration
	write_zeros(s, 8);                         // reserved
	s_wb16(s, 0);                              // layer
	s_wb16(s, 0);                              // alternate_group
	s_wb16(s, is_video(track) ? 0 : 0x0100);   // volume
	s_wb16(s, 0);                              // reserved
	write_matrix(s);
	s_wb32(s, track->width << 16);
	s_wb32(s, track->height << 16);
	box_end(s, start);
}

static void write_mdhd(struct serializer *s, const struct fmp4_track *track)
{
	int64_t start = fullbox_begin(s, "mdhd", 0, 0);
	s_wb32(s, 0); // creation_time
	s_wb32(s, 0); // modification_time
	s_wb32(s, track->timescale);
	s_wb32(s, 0);      // duration
	s_wb16(s, 0x55C4); // language: und
	s_wb16(s, 0);      // pre_defined
	box_end(s, start);
}

static void write_hdlr(struct serializer *s, const struct fmp4_track *track)
{
	const char *name = is_video(track) ? "VideoHandler" : "SoundHandler";
	int64_t start = fullbox_begin(s, "hdlr", 0, 0);
	s_wb32(s, 0); // pre_defined
	s_write(s, is_video(track) ? "vide" : "soun", 4);
	write_zeros(s, 12); // reserved
	s_write(s, name, strlen(name) + 1);
	box_end(s, start);
}

static void write_dinf(struct serializer *s)
{
	int64_t dinf = box_begin(s, "dinf");
	int64_t dref = fullbox_begin(s, "dref", 0, 0);
	s_wb32(s, 1); // entry_count
	box_end(s, fullbox_begin(s, "url ", 0, 1)); // media is in this file
	box_end(s, dref);
	box_end(s, dinf);
}

static void write_avc1(struct serializer *s, const struct fmp4_track *track)
{
	int64_t start = box_begin(s, "avc1");
	write_zeros(s, 6); // reserved
	s_wb16(s, 1);      // data_reference_index
	write_zeros(s, 16); // pre_defined, reserved
	s_wb16(s, (uint16_t)track->width);
	s_wb16(s, (uint16_t)track->height);
	s_wb32(s, 0x00480000); // horizresolution: 72 dpi
	s_wb32(s, 0x00480000); // vertresolution: 72 dpi
	s_wb32(s, 0);          // reserved
	s_wb16(s, 1);          // frame_count
	write_zeros(s, 32);    // compressorname
	s_wb16(s, 0x0018);     // depth
	s_wb16(s, 0xFFFF);     // pre_defined

	int64_t avcc = box_begin(s, "avcC");
	s_write(s, track->config, track->config_size);
	box_end(s, avcc);
	box_end(s, start);
}

static void write_audio_sample_entry(struct serializer *s, const struct fmp4_track *track)
{
	write_zeros(s, 6); // reserved
	s_wb16(s, 1);      // data_reference_index
	write_zeros(s, 8); // reserved
	s_wb16(s, (uint16_t)track->channels);
	s_wb16(s, 16); // samplesize
	s_wb16(s, 0);  // pre_defined
	s_wb16(s, 0);  // reserved
	s_wb32(s, track->sample_rate << 16);
}

/* ISO/IEC 14496-1 expandable descriptor size, always in the 4-byte form. */
static void write_descriptor(struct serializer *s, uint8_t tag, uint32_t size)
{
	s_w8(s, tag);
	for (int i = 3; i > 0; i--)
		s_w8(s, (uint8_t)((size >> (7 * i)) & 0x7F) | 0x80);
	s_w8(s, size & 0x7F);
}

static void write_mp4a(struct serializer *s, const struct fmp4_track *track)
{
	int64_t start = box_begin(s, "mp4a");
	write_audio_sample_entry(s, track);

	int64_t esds = fullbox_begin(s, "esds", 0, 0);
	uint32_t specific_info_size = 5 + (uint32_t)track->config_size;
	write_descriptor(s, 0x03, 3 + 5 + 13 + specific_info_size + 5 + 1); // ES_Descriptor
	s_wb16(s, (uint16_t)track->id);
	s_w8(s, 0);                                           // flags
	write_descriptor(s, 0x04, 13 + specific_info_size); // DecoderConfigDescriptor
	s_w8(s, 0x40);                                        // objectTypeIndication: AAC
	s_w8(s, 0x15);                                        // streamType: audio
	s_wb24(s, 0);                                         // bufferSizeDB
	s_wb32(s, track->bitrate);                            // maxBitrate
	s_wb32(s, track->bitrate);                            // avgBitrate
	write_descriptor(s, 0x05, (uint32_t)track->config_size); // DecoderSpecificInfo
	s_write(s, track->config, track->config_size);
	write_descriptor(s, 0x06, 1); // SLConfigDescriptor
	s_w8(s, 0x02);                // predefined: MP4
	box_end(s, esds);

	box_end(s, start);
}

static uint16_t read_le16(const uint8_t *p)
{
	return (uint16_t)(p[0] | (p[1] << 8));
}

static uint32_t read_le32(const uint8_t *p)
{
	return (uint32_t)p[0] | ((uint32_t)p[1] << 8) | ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}

/* Opus in ISO BMFF: dOps is the OpusHead with its fields made big-endian. */
static void write_opus(struct serializer *s, const struct fmp4_track *track)
{
	const uint8_t *head = track->config;
	uint8_t channels = head[9];
	uint8_t mapping_family = head[18];

	int64_t start = box_begin(s, "Opus");
	write_audio_sample_entry(s, track);

	int64_t dops = box_begin(s, "dOps");
	s_w8(s, 0); // version
	s_w8(s, channels);
	s_wb16(s, read_le16(head + 10)); // pre-skip
	s_wb32(s, read_le32(head + 12)); // input sample rate
	s_wb16(s, read_le16(head + 16)); // output gain
	s_w8(s, mapping_family);
	if (mapping_family && track->config_size >= 21u + channels)
		s_write(s, head + 19, 2 + channels); // stream count, coupled count, mapping
	box_end(s, dops);

	box_end(s, start);
}

static void write_stbl(struct serializer *s, const struct fmp4_track *track)
{
	int64_t stbl = box_begin(s, "stbl");

	int64_t stsd = fullbox_begin(s, "stsd", 0, 0);
	s_wb32(s, 1); // entry_count
	switch (track->codec) {
	case FMP4_H264:
		write_avc1(s, track);
		break;
	case FMP4_AAC:
		write_mp4a(s, track);
		break;
	case FMP4_OPUS:
		write_opus(s, track);
		break;
	}
	box_end(s, stsd);

	/* Fragmented: the sample tables are empty and every sample lives in a trun. */
	int64_t box = fullbox_begin(s, "stts", 0, 0);
	s_wb32(s, 0);
	box_end(s, box);
	box = fullbox_begin(s, "stsc", 0, 0);
	s_wb32(s, 0);
	box_end(s, box);
	box = fullbox_begin(s, "stsz", 0, 0);
	s_wb32(s, 0); // sample_size
	s_wb32(s, 0); // sample_count
	box_end(s, box);
	box = fullbox_begin(s, "stco", 0, 0);
	s_wb32(s, 0);
	box_end(s, box);

	box_end(s, stbl);
}

/* Encoders can start before zero (AAC and Opus priming): the track's media
 * timeline is shifted to start at zero, and the edit list shifts it back. */
static void write_edts(struct serializer *s, const struct fmp4_track *track)
{
	int64_t edts = box_begin(s, "edts");
	int64_t elst = fullbox_begin(s, "elst", 0, 0);
	s_wb32(s, 1); // entry_count
	s_wb32(s, 0); // segment_duration: the whole (fragmented) track
	s_wb32(s, (uint32_t)(track->dts_offset * track->timebase_num)); // media_time
	s_wb32(s, 0x00010000);                                           // media_rate
	box_end(s, elst);
	box_end(s, edts);
}

static void write_trak(struct serializer *s, const struct fmp4_track *track)
{
	int64_t trak = box_begin(s, "trak");
	write_tkhd(s, track);
	if (track->dts_offset)
		write_edts(s, track);

	int64_t mdia = box_begin(s, "mdia");
	write_mdhd(s, track);
	write_hdlr(s, track);

	int64_t minf = box_begin(s, "minf");
	if (is_video(track)) {
		int64_t vmhd = fullbox_begin(s, "vmhd", 0, 1);
		write_zeros(s, 8); // graphicsmode, opcolor
		box_end(s, vmhd);
	} else {
		int64_t smhd = fullbox_begin(s, "smhd", 0, 0);
		write_zeros(s, 4); // balance, reserved
		box_end(s, smhd);
	}
	write_dinf(s);
	write_stbl(s, track);
	box_end(s, minf);

	box_end(s, mdia);
	box_end(s, trak);
}

static void write_init(struct fmp4_mux *mux, struct serializer *s)
{
	int64_t ftyp = box_begin(s, "ftyp");
	s_write(s, "iso6", 4); // major_brand
	s_wb32(s, 0);          // minor_version
	s_write(s, "iso6", 4);
	s_write(s, "mp41", 4);
	box_end(s, ftyp);

	int64_t moov = box_begin(s, "moov");
	write_mvhd(s, (uint32_t)mux->tracks.num + 1);
	for (size_t i = 0; i < mux->tracks.num; i++)
		write_trak(s, &mux->tracks.array[i]);

	int64_t mvex = box_begin(s, "mvex");
	for (size_t i = 0; i < mux->tracks.num; i++) {
		int64_t trex = fullbox_begin(s, "trex", 0, 0);
		s_wb32(s, mux->tracks.array[i].id);
		s_wb32(s, 1); // default_sample_description_index
		s_wb32(s, 0); // default_sample_duration
		s_wb32(s, 0); // default_sample_size
		s_wb32(s, 0); // default_sample_flags
		box_end(s, trex);
	}
	box_end(s, mvex);
	box_end(s, moov);
}

static void complete_pending(struct fmp4_track *track, uint32_t duration)
{
	struct fmp4_sample *sample = da_push_back_new(track->samples);
	sample->packet = track->pending;
	sample->duration = duration;
	track->last_duration = duration;
	track->has_pending = false;
}

void fmp4_push(struct fmp4_mux *mux, size_t track_idx, struct encoder_packet *packet)
{
	struct fmp4_track *track = &mux->tracks.array[track_idx];
	if (!track->started) {
		track->started = true;
		track->dts_offset = packet->dts < 0 ? -packet->dts : 0;
	}
	if (track->has_pending) {
		int64_t delta = packet->dts - track->pending.dts;
		complete_pending(track, delta > 0 ? (uint32_t)(delta * track->timebase_num) : 0);
	}
	track->pending = *packet;
	track->has_pending = true;
}

static uint32_t sample_flags(const struct fmp4_track *track, const struct encoder_packet *packet)
{
	if (!is_video(track) || packet->keyframe)
		return SAMPLE_FLAGS_SYNC;
	return SAMPLE_FLAGS_NON_SYNC;
}

static void write_fragment(struct fmp4_mux *mux, struct serializer *s, bool force_init)
{
	if (!mux->init_written) {
		bool all_started = true;
		for (size_t i = 0; i < mux->tracks.num; i++)
			all_started = all_started && mux->tracks.array[i].started;
		if (!all_started && !force_init)
			return;
		write_init(mux, s);
		mux->init_written = true;
	}

	int64_t data_offset_pos[FMP4_MAX_TRACKS];
	bool any = false;
	for (size_t i = 0; i < mux->tracks.num; i++)
		any = any || mux->tracks.array[i].samples.num;
	if (!any)
		return;

	int64_t moof = box_begin(s, "moof");
	int64_t mfhd = fullbox_begin(s, "mfhd", 0, 0);
	s_wb32(s, ++mux->sequence);
	box_end(s, mfhd);

	for (size_t i = 0; i < mux->tracks.num; i++) {
		struct fmp4_track *track = &mux->tracks.array[i];
		if (!track->samples.num)
			continue;

		int64_t traf = box_begin(s, "traf");
		int64_t tfhd = fullbox_begin(s, "tfhd", 0, TFHD_DEFAULT_BASE_IS_MOOF);
		s_wb32(s, track->id);
		box_end(s, tfhd);

		uint64_t first_dts = (uint64_t)(track->samples.array[0].packet.dts + track->dts_offset);
		int64_t tfdt = fullbox_begin(s, "tfdt", 1, 0);
		s_wb64(s, first_dts * track->timebase_num);
		box_end(s, tfdt);

		/* Version 1: signed composition offsets. */
		int64_t trun = fullbox_begin(s, "trun", 1, TRUN_FLAGS);
		s_wb32(s, (uint32_t)track->samples.num);
		data_offset_pos[i] = serializer_get_pos(s);
		s_wb32(s, 0); // data_offset, patched below
		for (size_t j = 0; j < track->samples.num; j++) {
			const struct fmp4_sample *sample = &track->samples.array[j];
			int64_t offset = (sample->packet.pts - sample->packet.dts) * track->timebase_num;
			s_wb32(s, sample->duration);
			s_wb32(s, (uint32_t)sample->packet.size);
			s_wb32(s, sample_flags(track, &sample->packet));
			s_wb32(s, (uint32_t)(int32_t)offset);
		}
		box_end(s, trun);
		box_end(s, traf);
	}
	box_end(s, moof);

	/* Sample data follows the moof in track order; trun offsets are relative to the moof. */
	uint32_t data_offset = (uint32_t)(serializer_get_pos(s) - moof) + 8;
	int64_t mdat = box_begin(s, "mdat");
	for (size_t i = 0; i < mux->tracks.num; i++) {
		struct fmp4_track *track = &mux->tracks.array[i];
		if (!track->samples.num)
			continue;

		patch_u32(s, data_offset_pos[i], data_offset);
		for (size_t j = 0; j < track->samples.num; j++) {
			struct encoder_packet *packet = &track->samples.array[j].packet;
			s_write(s, packet->data, packet->size);
			data_offset += (uint32_t)packet->size;
			obs_encoder_packet_release(packet);
		}
		da_resize(track->samples, 0);
	}
	box_end(s, mdat);
}

void fmp4_write_fragment(struct fmp4_mux *mux, struct serializer *s)
{
	write_fragment(mux, s, false);
}

static uint32_t default_duration(const struct fmp4_track *track)
{
	switch (track->codec) {
	case FMP4_AAC:
		return 1024;
	case FMP4_OPUS:
		return 960;
	default:
		return track->timebase_num; // one frame
	}
}

void fmp4_finish(struct fmp4_mux *mux, struct serializer *s)
{
	for (size_t i = 0; i < mux->tracks.num; i++) {
		struct fmp4_track *track = &mux->tracks.array[i];
		if (track->has_pending)
			complete_pending(track, track->last_duration ? track->last_duration : default_duration(track));
	}
	write_fragment(mux, s, true);
}
