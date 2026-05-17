---
title: place.stream.media.viewCount
description: Reference for the place.stream.media.viewCount lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A streamplace node's report of view counts for one place.stream.video over a closed time window. Published in the reporting node's server repo (not the streamer's), so a video served by multiple nodes accumulates multiple records — consumers are expected to sum across trusted reporters. The rkey is conventionally `<sha256(videoUri || windowStart)>` truncated, so re-running the aggregator over the same window is idempotent. The methodology field documents the floor used to count a session (e.g. "any-segment" = any sid that fetched ≥ threshold segment_requests).

**Record Key:** `any`

**Record Properties:**

| Name                | Type      | Req'd | Description                                                                                                                                                                                                                                                           | Constraints        |
| ------------------- | --------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `video`             | `string`  | ✅    | AT-URI of the place.stream.video this count is for.                                                                                                                                                                                                                   | Format: `at-uri`   |
| `count`             | `integer` | ✅    | Number of distinct sessions that qualified as a view over [windowStart, windowEnd) by the stated methodology.                                                                                                                                                         | Min: 0             |
| `windowStart`       | `string`  | ✅    | Inclusive lower bound of the aggregation window.                                                                                                                                                                                                                      | Format: `datetime` |
| `windowEnd`         | `string`  | ✅    | Exclusive upper bound of the aggregation window.                                                                                                                                                                                                                      | Format: `datetime` |
| `methodology`       | `string`  | ✅    | Identifier for the counting algorithm. "any-segment" means: a distinct sid that fetched at least `thresholdSegments` segments for this video over the window. Future methodologies (e.g. "ms-from-metafile") will use new tags so older records remain interpretable. |                    |
| `thresholdSegments` | `integer` | ❌    | Floor on segment_request count per (sid, video) for the "any-segment" methodology. Defaults to 1.                                                                                                                                                                     | Min: 0             |
| `indexedAt`         | `string`  | ✅    | When the reporting node ran this aggregation. Useful for ordering successive reports.                                                                                                                                                                                 | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.viewCount",
  "defs": {
    "main": {
      "type": "record",
      "description": "A streamplace node's report of view counts for one place.stream.video over a closed time window. Published in the reporting node's server repo (not the streamer's), so a video served by multiple nodes accumulates multiple records — consumers are expected to sum across trusted reporters. The rkey is conventionally `<sha256(videoUri || windowStart)>` truncated, so re-running the aggregator over the same window is idempotent. The methodology field documents the floor used to count a session (e.g. \"any-segment\" = any sid that fetched ≥ threshold segment_requests).",
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
          "indexedAt": {
            "type": "string",
            "format": "datetime",
            "description": "When the reporting node ran this aggregation. Useful for ordering successive reports."
          }
        }
      }
    }
  }
}
```
