---
title: place.stream.media.viewCount
description: Reference for the place.stream.media.viewCount lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A streamplace node's report of view counts for one place.stream.video over a closed time window. Published in the reporting node's server repo (not the streamer's), so a video served by multiple nodes accumulates multiple records — consumers are expected to sum across trusted reporters. The rkey is conventionally `<windowStart-as-tid>-<video-rkey>` so re-running the aggregator over the same window is idempotent. The methodology field documents the floor used to count a session (e.g. "any-segment" = any sid that fetched ≥ threshold segment_requests); the embedded `tracks` array carries objective per-track bytes + duration totals that don't depend on the methodology.

**Record Key:** `any`

**Record Properties:**

| Name                | Type                                  | Req'd | Description                                                                                                                                                                                                                                                                | Constraints        |
| ------------------- | ------------------------------------- | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `video`             | `string`                              | ✅    | AT-URI of the place.stream.video this count is for.                                                                                                                                                                                                                        | Format: `at-uri`   |
| `count`             | `integer`                             | ✅    | Number of distinct sessions that qualified as a view over [windowStart, windowEnd) by the stated methodology.                                                                                                                                                              | Min: 0             |
| `windowStart`       | `string`                              | ✅    | Inclusive lower bound of the aggregation window.                                                                                                                                                                                                                           | Format: `datetime` |
| `windowEnd`         | `string`                              | ✅    | Exclusive upper bound of the aggregation window.                                                                                                                                                                                                                           | Format: `datetime` |
| `methodology`       | `string`                              | ✅    | Identifier for the counting algorithm. "any-segment" means: a distinct sid that fetched at least `thresholdSegments` segments for this video over the window. Future methodologies (e.g. "ms-from-metafile") will use new tags so older records remain interpretable.      |                    |
| `thresholdSegments` | `integer`                             | ❌    | Floor on segment_request count per (sid, video) for the "any-segment" methodology. Defaults to 1.                                                                                                                                                                          | Min: 0             |
| `tracks`            | Array of [`#trackUsage`](#trackusage) | ❌    | Per-track totals of bytes + playback duration actually transferred over the window. Computed by intersecting each segment_request's HTTP Range with the metafile's per-track byte layout, so the numbers are objective regardless of the chosen view-counting methodology. |                    |
| `indexedAt`         | `string`                              | ✅    | When the reporting node ran this aggregation. Useful for ordering successive reports.                                                                                                                                                                                      | Format: `datetime` |

---

<a name="trackusage"></a>

### `trackUsage`

**Type:** `object`

One row of the tracks array: bytes + duration transferred for a single muxlTrack inside the video's MUXL container over the window. trackId matches the muxlTrack record's `trackId` field.

**Properties:**

| Name         | Type      | Req'd | Description                                                                                                                                                                                                             | Constraints |
| ------------ | --------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `trackId`    | `string`  | ✅    | Stringified u32 matching the MUXL container's per-track id.                                                                                                                                                             |             |
| `bytes`      | `integer` | ✅    | Total bytes served from this track's byte ranges over the window. Sum across attributed segment_requests' Range intersections with the track's segment offsets.                                                         | Min: 0      |
| `durationMs` | `integer` | ✅    | Total playback duration served from this track, in milliseconds. Per HLS segment in the range: (overlap bytes / segment bytes) \* segment duration, so partial-segment fetches credit a proportional share of duration. | Min: 0      |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.viewCount",
  "defs": {
    "main": {
      "type": "record",
      "description": "A streamplace node's report of view counts for one place.stream.video over a closed time window. Published in the reporting node's server repo (not the streamer's), so a video served by multiple nodes accumulates multiple records — consumers are expected to sum across trusted reporters. The rkey is conventionally `<windowStart-as-tid>-<video-rkey>` so re-running the aggregator over the same window is idempotent. The methodology field documents the floor used to count a session (e.g. \"any-segment\" = any sid that fetched ≥ threshold segment_requests); the embedded `tracks` array carries objective per-track bytes + duration totals that don't depend on the methodology.",
      "key": "any",
      "record": {
        "type": "object",
        "required": [
          "video",
          "count",
          "windowStart",
          "windowEnd",
          "methodology",
          "indexedAt"
        ],
        "properties": {
          "video": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.video this count is for."
          },
          "count": {
            "type": "integer",
            "minimum": 0,
            "description": "Number of distinct sessions that qualified as a view over [windowStart, windowEnd) by the stated methodology."
          },
          "windowStart": {
            "type": "string",
            "format": "datetime",
            "description": "Inclusive lower bound of the aggregation window."
          },
          "windowEnd": {
            "type": "string",
            "format": "datetime",
            "description": "Exclusive upper bound of the aggregation window."
          },
          "methodology": {
            "type": "string",
            "description": "Identifier for the counting algorithm. \"any-segment\" means: a distinct sid that fetched at least `thresholdSegments` segments for this video over the window. Future methodologies (e.g. \"ms-from-metafile\") will use new tags so older records remain interpretable."
          },
          "thresholdSegments": {
            "type": "integer",
            "minimum": 0,
            "description": "Floor on segment_request count per (sid, video) for the \"any-segment\" methodology. Defaults to 1."
          },
          "tracks": {
            "type": "array",
            "description": "Per-track totals of bytes + playback duration actually transferred over the window. Computed by intersecting each segment_request's HTTP Range with the metafile's per-track byte layout, so the numbers are objective regardless of the chosen view-counting methodology.",
            "items": {
              "type": "ref",
              "ref": "#trackUsage"
            }
          },
          "indexedAt": {
            "type": "string",
            "format": "datetime",
            "description": "When the reporting node ran this aggregation. Useful for ordering successive reports."
          }
        }
      }
    },
    "trackUsage": {
      "type": "object",
      "description": "One row of the tracks array: bytes + duration transferred for a single muxlTrack inside the video's MUXL container over the window. trackId matches the muxlTrack record's `trackId` field.",
      "required": ["trackId", "bytes", "durationMs"],
      "properties": {
        "trackId": {
          "type": "string",
          "description": "Stringified u32 matching the MUXL container's per-track id."
        },
        "bytes": {
          "type": "integer",
          "minimum": 0,
          "description": "Total bytes served from this track's byte ranges over the window. Sum across attributed segment_requests' Range intersections with the track's segment offsets."
        },
        "durationMs": {
          "type": "integer",
          "minimum": 0,
          "description": "Total playback duration served from this track, in milliseconds. Per HLS segment in the range: (overlap bytes / segment bytes) * segment duration, so partial-segment fetches credit a proportional share of duration."
        }
      }
    }
  }
}
```
