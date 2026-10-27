#pragma once

#include <obs.h>

/* AV1 in ISO BMFF (AV1-ISOBMFF), adapted from obs-studio's
 * plugins/obs-outputs/rtmp-av1.c, which libobs does not export. */

/* Builds an AV1CodecConfigurationRecord (av1C payload) from an encoder's
 * extra data: its sequence header OBU, or a ready-made record. Returns the
 * size of the bmalloc'd *header, 0 when the extra data is unusable. */
size_t av1_parse_header(uint8_t **header, const uint8_t *data, size_t size);
/* Copies a temporal unit as an ISO BMFF sample: without the temporal
 * delimiter, redundant frame header and tile list OBUs it must not carry. */
void av1_parse_packet(struct encoder_packet *dst, const struct encoder_packet *src);
