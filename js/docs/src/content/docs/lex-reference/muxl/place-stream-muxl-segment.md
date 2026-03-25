---
title: place.stream.muxl.segment
description: Reference for the place.stream.muxl.segment lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

One or more MUXL segments for a stream. Initially one record per GOP (all tracks bundled). Records can be compacted by merging multiple GOPs into a single record with a combined blob.

**Record Key:** `tid`

**Record Properties:**

| Name      | Type                                                                                                                                   | Req'd | Description                                                                        | Constraints |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ---------------------------------------------------------------------------------- | ----------- |
| `catalog` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | The place.stream.muxl.catalog record for this segment's track configuration.       |             |
| `tracks`  | Array of [`#trackSegment`](#tracksegment)                                                                                              | ✅    | Per-track segment data. One entry per track per GOP; multiple GOPs when compacted. |             |

---

<a name="tracksegment"></a>

### `trackSegment`

**Type:** `object`

**Properties:**

| Name             | Type      | Req'd | Description                                                           | Constraints         |
| ---------------- | --------- | ----- | --------------------------------------------------------------------- | ------------------- |
| `trackId`        | `integer` | ✅    | MP4 track ID.                                                         |                     |
| `sequenceNumber` | `integer` | ✅    | GOP sequence number (1-based, ascending). Used for ordering segments. |                     |
| `durationTicks`  | `integer` | ✅    | Total duration in timescale ticks (see catalog for timescale).        |                     |
| `sampleCount`    | `integer` | ✅    | Number of samples (frames) in this segment.                           |                     |
| `data`           | `blob`    | ✅    | The moof+mdat bytes for this track segment.                           | Accept: `video/mp4` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.muxl.segment",
  "defs": {
    "main": {
      "type": "record",
      "description": "One or more MUXL segments for a stream. Initially one record per GOP (all tracks bundled). Records can be compacted by merging multiple GOPs into a single record with a combined blob.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["catalog", "tracks"],
        "properties": {
          "catalog": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The place.stream.muxl.catalog record for this segment's track configuration."
          },
          "tracks": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "#trackSegment"
            },
            "description": "Per-track segment data. One entry per track per GOP; multiple GOPs when compacted."
          }
        }
      }
    },
    "trackSegment": {
      "type": "object",
      "required": [
        "trackId",
        "sequenceNumber",
        "durationTicks",
        "sampleCount",
        "data"
      ],
      "properties": {
        "trackId": {
          "type": "integer",
          "description": "MP4 track ID."
        },
        "sequenceNumber": {
          "type": "integer",
          "description": "GOP sequence number (1-based, ascending). Used for ordering segments."
        },
        "durationTicks": {
          "type": "integer",
          "description": "Total duration in timescale ticks (see catalog for timescale)."
        },
        "sampleCount": {
          "type": "integer",
          "description": "Number of samples (frames) in this segment."
        },
        "data": {
          "type": "blob",
          "accept": ["video/mp4"],
          "description": "The moof+mdat bytes for this track segment."
        }
      }
    }
  }
}
```
