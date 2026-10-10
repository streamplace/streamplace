#pragma once

#include <obs.h>
#include <util/darray.h>
#include <util/serializer.h>

/* Fragmented MP4 muxer for a live push: one init segment (ftyp+moov) once every
 * track has a sample, then a moof+mdat fragment each time the caller asks.
 * Every track holds back its newest sample until the next one arrives, so each
 * sample's duration is the real gap to its successor (OBS drops frames under
 * load). */

#define FMP4_MAX_TRACKS (MAX_OUTPUT_VIDEO_ENCODERS + MAX_OUTPUT_AUDIO_ENCODERS)

enum fmp4_codec {
	FMP4_H264,
	FMP4_AAC,
	FMP4_OPUS,
};

struct fmp4_sample {
	struct encoder_packet packet;
	uint32_t duration;
};

struct fmp4_track {
	uint32_t id;
	enum fmp4_codec codec;
	/* Media timescale is the encoder's timebase denominator; packet
	 * timestamps are multiplied by the numerator. */
	uint32_t timescale;
	uint32_t timebase_num;
	uint32_t width, height;
	uint32_t channels, sample_rate;
	uint32_t bitrate;
	/* avcC payload for H.264, AudioSpecificConfig for AAC, OpusHead for Opus. */
	uint8_t *config;
	size_t config_size;

	bool started;
	/* Packets before zero (encoder priming), in timebase units; undone by an edit list. */
	int64_t dts_offset;
	bool has_pending;
	struct encoder_packet pending;
	uint32_t last_duration;
	DARRAY(struct fmp4_sample) samples;
};

struct fmp4_mux {
	DARRAY(struct fmp4_track) tracks;
	uint32_t sequence;
	bool init_written;
};

void fmp4_free(struct fmp4_mux *mux);

/* Copies the track description and its config bytes. Returns the track index. */
size_t fmp4_add_track(struct fmp4_mux *mux, const struct fmp4_track *info);
/* Takes ownership of a packet whose data was ref'd or allocated by libobs.
 * Video packets must already be length-prefixed (AVCC). */
void fmp4_push(struct fmp4_mux *mux, size_t track_idx, struct encoder_packet *packet);
/* Writes the init segment once possible, then every sample with a known
 * duration; no-op when there is nothing to write. */
void fmp4_write_fragment(struct fmp4_mux *mux, struct serializer *s);
/* Writes the held-back samples, reusing each track's previous duration. */
void fmp4_finish(struct fmp4_mux *mux, struct serializer *s);
