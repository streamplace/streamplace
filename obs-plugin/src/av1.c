#include "av1.h"

#include <util/array-serializer.h>

#define OBU_SEQUENCE_HEADER 1
#define OBU_TEMPORAL_DELIMITER 2
#define OBU_METADATA 5
#define OBU_REDUNDANT_FRAME_HEADER 7
#define OBU_TILE_LIST 8

#define PROFILE_MAIN 0
#define PROFILE_HIGH 1
#define PROFILE_PROFESSIONAL 2

struct bits {
	const uint8_t *buf;
	size_t size, pos; /* in bits */
};

/* Reads past the end as zeros; callers check pos against size afterwards. */
static uint32_t get_bits(struct bits *b, unsigned n)
{
	uint32_t v = 0;
	for (unsigned i = 0; i < n; i++, b->pos++) {
		uint32_t bit = b->pos < b->size ? (b->buf[b->pos >> 3] >> (7 - (b->pos & 7))) & 1 : 0;
		v = (v << 1) | bit;
	}
	return v;
}

static void skip_uvlc(struct bits *b)
{
	unsigned leading_zeros = 0;
	while (b->pos < b->size && !get_bits(b, 1))
		leading_zeros++;
	if (leading_zeros < 32)
		get_bits(b, leading_zeros);
}

/* Returns the OBU's total length, or 0 when it does not fit in size. */
static size_t parse_obu(const uint8_t *buf, size_t size, int *type, size_t *payload_pos, size_t *payload_size)
{
	if (!size || (buf[0] & 0x80))
		return 0;
	*type = (buf[0] >> 3) & 0xf;
	bool has_size = buf[0] & 0x02;
	size_t pos = (buf[0] & 0x04) ? 2 : 1;
	if (pos > size)
		return 0;

	uint64_t len = size - pos;
	if (has_size) {
		len = 0;
		for (int i = 0; i < 8; i++) {
			if (pos >= size)
				return 0;
			uint8_t byte = buf[pos++];
			len |= (uint64_t)(byte & 0x7f) << (i * 7);
			if (!(byte & 0x80))
				break;
		}
	}
	if (len > size - pos)
		return 0;
	*payload_pos = pos;
	*payload_size = (size_t)len;
	return pos + (size_t)len;
}

struct seq_params {
	uint8_t profile, level, tier, bitdepth, monochrome;
	uint8_t chroma_subsampling_x, chroma_subsampling_y, chroma_sample_position;
};

static void parse_color_config(struct seq_params *p, struct bits *b)
{
	bool high_bitdepth = get_bits(b, 1);
	bool twelve_bit = p->profile == PROFILE_PROFESSIONAL && high_bitdepth && get_bits(b, 1);
	p->bitdepth = twelve_bit ? 12 : high_bitdepth ? 10 : 8;
	p->monochrome = p->profile == PROFILE_HIGH ? 0 : get_bits(b, 1);

	uint32_t primaries = 2, transfer = 2, matrix = 2;
	if (get_bits(b, 1)) { // color_description_present_flag
		primaries = get_bits(b, 8);
		transfer = get_bits(b, 8);
		matrix = get_bits(b, 8);
	}

	if (p->monochrome) {
		get_bits(b, 1); // color_range
		p->chroma_subsampling_x = p->chroma_subsampling_y = 1;
		return;
	}
	if (primaries == 1 && transfer == 13 && matrix == 0) { // sRGB
		p->chroma_subsampling_x = p->chroma_subsampling_y = 0;
		return;
	}
	get_bits(b, 1); // color_range
	if (p->profile == PROFILE_MAIN) {
		p->chroma_subsampling_x = p->chroma_subsampling_y = 1;
	} else if (p->profile == PROFILE_HIGH) {
		p->chroma_subsampling_x = p->chroma_subsampling_y = 0;
	} else if (twelve_bit) {
		p->chroma_subsampling_x = get_bits(b, 1);
		p->chroma_subsampling_y = p->chroma_subsampling_x ? get_bits(b, 1) : 0;
	} else {
		p->chroma_subsampling_x = 1;
		p->chroma_subsampling_y = 0;
	}
	if (p->chroma_subsampling_x && p->chroma_subsampling_y)
		p->chroma_sample_position = get_bits(b, 2);
}

/* AV1 spec 5.5.1 sequence_header_obu, up to and including color_config. */
static bool parse_sequence_header(struct seq_params *p, const uint8_t *buf, size_t size)
{
	struct bits b = {.buf = buf, .size = size * 8};
	memset(p, 0, sizeof(*p));

	p->profile = get_bits(&b, 3);
	get_bits(&b, 1); // still_picture
	bool reduced_still_picture_header = get_bits(&b, 1);
	if (reduced_still_picture_header) {
		p->level = get_bits(&b, 5);
	} else {
		bool decoder_model_info_present = false;
		uint32_t buffer_delay_length = 0;
		if (get_bits(&b, 1)) { // timing_info_present_flag
			get_bits(&b, 32);  // num_units_in_display_tick
			get_bits(&b, 32);  // time_scale
			if (get_bits(&b, 1)) // equal_picture_interval
				skip_uvlc(&b);
			decoder_model_info_present = get_bits(&b, 1);
			if (decoder_model_info_present) {
				buffer_delay_length = get_bits(&b, 5) + 1;
				get_bits(&b, 32); // num_units_in_decoding_tick
				get_bits(&b, 10); // buffer_removal_time_length_minus_1, frame_presentation_time_length_minus_1
			}
		}
		bool initial_display_delay_present = get_bits(&b, 1);
		uint32_t operating_points = get_bits(&b, 5) + 1;
		for (uint32_t i = 0; i < operating_points; i++) {
			get_bits(&b, 12); // operating_point_idc
			uint8_t level = get_bits(&b, 5);
			uint8_t tier = level > 7 ? get_bits(&b, 1) : 0;
			if (decoder_model_info_present && get_bits(&b, 1)) {
				get_bits(&b, buffer_delay_length); // decoder_buffer_delay
				get_bits(&b, buffer_delay_length); // encoder_buffer_delay
				get_bits(&b, 1);                   // low_delay_mode_flag
			}
			if (initial_display_delay_present && get_bits(&b, 1))
				get_bits(&b, 4);
			if (i == 0) {
				p->level = level;
				p->tier = tier;
			}
		}
	}

	uint32_t width_bits = get_bits(&b, 4) + 1;
	uint32_t height_bits = get_bits(&b, 4) + 1;
	get_bits(&b, width_bits);  // max_frame_width_minus_1
	get_bits(&b, height_bits); // max_frame_height_minus_1
	if (!reduced_still_picture_header && get_bits(&b, 1)) // frame_id_numbers_present_flag
		get_bits(&b, 7);
	get_bits(&b, 3); // use_128x128_superblock, enable_filter_intra, enable_intra_edge_filter

	if (!reduced_still_picture_header) {
		get_bits(&b, 4); // interintra, masked_compound, warped_motion, dual_filter
		bool enable_order_hint = get_bits(&b, 1);
		if (enable_order_hint)
			get_bits(&b, 2); // enable_jnt_comp, enable_ref_frame_mvs
		uint32_t force_screen_content_tools = get_bits(&b, 1) ? 2 : get_bits(&b, 1);
		if (force_screen_content_tools && !get_bits(&b, 1)) // seq_choose_integer_mv
			get_bits(&b, 1);                             // seq_force_integer_mv
		if (enable_order_hint)
			get_bits(&b, 3); // order_hint_bits_minus_1
	}
	get_bits(&b, 3); // enable_superres, enable_cdef, enable_restoration

	parse_color_config(p, &b);
	get_bits(&b, 1); // film_grain_params_present
	return b.pos <= b.size;
}

size_t av1_parse_header(uint8_t **header, const uint8_t *data, size_t size)
{
	/* Already an AV1CodecConfigurationRecord: marker set, version 1. */
	if (size >= 4 && data[0] == 0x81) {
		*header = bmemdup(data, size);
		return size;
	}

	struct seq_params p;
	const uint8_t *seq = NULL, *meta = NULL;
	size_t seq_size = 0, meta_size = 0;
	while (size > 0) {
		int type;
		size_t payload_pos, payload_size;
		size_t len = parse_obu(data, size, &type, &payload_pos, &payload_size);
		if (!len)
			return 0;
		if (type == OBU_SEQUENCE_HEADER) {
			if (seq || !parse_sequence_header(&p, data + payload_pos, payload_size))
				return 0;
			seq = data;
			seq_size = len;
		} else if (type == OBU_METADATA) {
			meta = data;
			meta_size = len;
		}
		data += len;
		size -= len;
	}
	if (!seq)
		return 0;

	uint8_t record[4] = {
		0x81, // marker, version 1
		(uint8_t)(p.profile << 5 | p.level),
		(uint8_t)(p.tier << 7 | (p.bitdepth > 8) << 6 | (p.bitdepth == 12) << 5 | p.monochrome << 4 |
			  p.chroma_subsampling_x << 3 | p.chroma_subsampling_y << 2 | p.chroma_sample_position),
		0, // no initial_presentation_delay
	};

	struct array_output_data output;
	struct serializer s;
	array_output_serializer_init(&s, &output);
	s_write(&s, record, sizeof(record));
	s_write(&s, seq, seq_size);
	if (meta)
		s_write(&s, meta, meta_size);
	*header = output.bytes.array;
	return output.bytes.num;
}

void av1_parse_packet(struct encoder_packet *dst, const struct encoder_packet *src)
{
	/* libobs packet data is preceded by its reference count. */
	struct array_output_data output;
	struct serializer s;
	long ref = 1;
	array_output_serializer_init(&s, &output);
	s_write(&s, &ref, sizeof(ref));

	const uint8_t *data = src->data;
	size_t size = src->size;
	while (size > 0) {
		int type;
		size_t payload_pos, payload_size;
		size_t len = parse_obu(data, size, &type, &payload_pos, &payload_size);
		if (!len)
			break;
		if (type != OBU_TEMPORAL_DELIMITER && type != OBU_REDUNDANT_FRAME_HEADER && type != OBU_TILE_LIST)
			s_write(&s, data, len);
		data += len;
		size -= len;
	}

	*dst = *src;
	dst->data = output.bytes.array + sizeof(ref);
	dst->size = output.bytes.num - sizeof(ref);
}
