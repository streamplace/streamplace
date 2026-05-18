---
title: place.stream.media.viewCount
description: Reference for the place.stream.media.viewCount lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A streamplace node's report of view counts for one place.stream.video over a closed time window. Published in the reporting node's server repo (not the streamer's), so a video served by multiple nodes accumulates multiple records — consumers are expected to sum across trusted reporters. The rkey is conventionally `<windowStart-as-tid>-<video-rkey>` so re-running the aggregator over the same window is idempotent. Counts represent the reporting node's best effort given the data it has; the `tracks` array carries the objective byte / duration totals it observed.

**Record Key:** `any`

**Record Properties:**

| Name          | Type                                  | Req'd | Description                                                                                                                                                                                                                                                                                                 | Constraints        |
| ------------- | ------------------------------------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `video`       | `string`                              | ✅    | AT-URI of the place.stream.video this count is for.                                                                                                                                                                                                                                                         | Format: `at-uri`   |
| `count`       | `integer`                             | ✅    | Number of distinct sessions the reporting node observed as a view over [windowStart, windowEnd).                                                                                                                                                                                                            | Min: 0             |
| `windowStart` | `string`                              | ✅    | Inclusive lower bound of the aggregation window.                                                                                                                                                                                                                                                            | Format: `datetime` |
| `windowEnd`   | `string`                              | ✅    | Exclusive upper bound of the aggregation window.                                                                                                                                                                                                                                                            | Format: `datetime` |
| `tracks`      | Array of [`#trackUsage`](#trackusage) | ❌    | Per-track totals of bytes + playback duration the reporting node served over the window. Each entry references the place.stream.media.track record whose bytes were transferred, so user-contributed tracks (transcripts, transcodes published by other accounts) attribute naturally to their own records. |                    |
| `indexedAt`   | `string`                              | ✅    | When the reporting node ran this aggregation. Useful for ordering successive reports.                                                                                                                                                                                                                       | Format: `datetime` |

---

<a name="trackusage"></a>

### `trackUsage`

**Type:** `object`

One row of the tracks array: bytes + duration transferred for a single place.stream.media.track record over the window.

**Properties:**

| Name         | Type                                                                                                                                   | Req'd | Description                                                                                                                                                                                                             | Constraints |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `track`      | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | Strong reference to the place.stream.media.track record whose bytes were transferred.                                                                                                                                   |             |
| `bytes`      | `integer`                                                                                                                              | ✅    | Total bytes served from this track over the window. Sum across attributed segment_requests' Range intersections with the track's segment offsets in the metafile.                                                       | Min: 0      |
| `durationMs` | `integer`                                                                                                                              | ✅    | Total playback duration served from this track, in milliseconds. Per HLS segment in the range: (overlap bytes / segment bytes) \* segment duration, so partial-segment fetches credit a proportional share of duration. | Min: 0      |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.viewCount",
  "defs": {
    "main": {
      "type": "record",
      "description": "A streamplace node's report of view counts for one place.stream.video over a closed time window. Published in the reporting node's server repo (not the streamer's), so a video served by multiple nodes accumulates multiple records — consumers are expected to sum across trusted reporters. The rkey is conventionally `<windowStart-as-tid>-<video-rkey>` so re-running the aggregator over the same window is idempotent. Counts represent the reporting node's best effort given the data it has; the `tracks` array carries the objective byte / duration totals it observed.",
      "key": "any",
      "record": {
        "type": "object",
        "required": ["video", "count", "windowStart", "windowEnd", "indexedAt"],
        "properties": {
          "video": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.video this count is for."
          },
          "count": {
            "type": "integer",
            "minimum": 0,
            "description": "Number of distinct sessions the reporting node observed as a view over [windowStart, windowEnd)."
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
          "tracks": {
            "type": "array",
            "description": "Per-track totals of bytes + playback duration the reporting node served over the window. Each entry references the place.stream.media.track record whose bytes were transferred, so user-contributed tracks (transcripts, transcodes published by other accounts) attribute naturally to their own records.",
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
      "description": "One row of the tracks array: bytes + duration transferred for a single place.stream.media.track record over the window.",
      "required": ["track", "bytes", "durationMs"],
      "properties": {
        "track": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef",
          "description": "Strong reference to the place.stream.media.track record whose bytes were transferred."
        },
        "bytes": {
          "type": "integer",
          "minimum": 0,
          "description": "Total bytes served from this track over the window. Sum across attributed segment_requests' Range intersections with the track's segment offsets in the metafile."
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
