---
title: place.stream.media.getVideo
description: Reference for the place.stream.media.getVideo lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a hydrated view of a place.stream.video record — the record itself plus author info plus aggregated view counts summed across every reporting node we've indexed. View counts come from place.stream.media.viewCount records; consumers see one number per metric, with the underlying multi-reporter detail collapsed away.

**Parameters:**

| Name  | Type     | Req'd | Description                              | Constraints      |
| ----- | -------- | ----- | ---------------------------------------- | ---------------- |
| `uri` | `string` | ✅    | AT-URI of the place.stream.video record. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** [`#videoView`](#videoview)

**Possible Errors:**

- `VideoNotFound`: No record indexed at the supplied AT-URI.

---

<a name="videoview"></a>

### `videoView`

**Type:** `object`

**Properties:**

| Name         | Type                                                                                                                                             | Req'd | Description                                                                                                                                                                                                                    | Constraints      |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------- |
| `uri`        | `string`                                                                                                                                         | ✅    |                                                                                                                                                                                                                                | Format: `at-uri` |
| `cid`        | `string`                                                                                                                                         | ✅    |                                                                                                                                                                                                                                | Format: `cid`    |
| `author`     | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                                                                                                                                                                                                                |                  |
| `record`     | `unknown`                                                                                                                                        | ✅    |                                                                                                                                                                                                                                |                  |
| `viewCounts` | [`#viewCountSummary`](#viewcountsummary)                                                                                                         | ✅    | Aggregated view counts across every indexed reporter. Always present; zero-valued (count=0, reporters=0) when no place.stream.media.viewCount records have been observed yet, so consumers can render a count unconditionally. |                  |
| `tracks`     | Array of [`place.stream.media.track#trackView`](/lex-reference/place-stream-media-track#trackview)                                               | ❌    |                                                                                                                                                                                                                                |                  |

---

<a name="viewcountsummary"></a>

### `viewCountSummary`

**Type:** `object`

Sums across every place.stream.media.viewCount record indexed for this video, regardless of reporter or window. The underlying records are window-bounded; the consumer sees a running total.

**Properties:**

| Name         | Type      | Req'd | Description                                                                                              | Constraints |
| ------------ | --------- | ----- | -------------------------------------------------------------------------------------------------------- | ----------- |
| `count`      | `integer` | ✅    | Sum of `count` across every indexed report.                                                              | Min: 0      |
| `bytes`      | `integer` | ✅    | Sum of bytes transferred across every track row in every indexed report.                                 | Min: 0      |
| `durationMs` | `integer` | ✅    | Sum of playback duration (ms) across every track row in every indexed report.                            | Min: 0      |
| `reporters`  | `integer` | ✅    | Number of distinct streamplace nodes that have contributed at least one viewCount record for this video. | Min: 0      |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.getVideo",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a hydrated view of a place.stream.video record — the record itself plus author info plus aggregated view counts summed across every reporting node we've indexed. View counts come from place.stream.media.viewCount records; consumers see one number per metric, with the underlying multi-reporter detail collapsed away.",
      "parameters": {
        "type": "params",
        "required": ["uri"],
        "properties": {
          "uri": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.video record."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "ref",
          "ref": "#videoView"
        }
      },
      "errors": [
        {
          "name": "VideoNotFound",
          "description": "No record indexed at the supplied AT-URI."
        }
      ]
    },
    "videoView": {
      "type": "object",
      "required": ["uri", "cid", "author", "record", "viewCounts"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "author": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic"
        },
        "record": {
          "type": "unknown"
        },
        "viewCounts": {
          "type": "ref",
          "ref": "#viewCountSummary",
          "description": "Aggregated view counts across every indexed reporter. Always present; zero-valued (count=0, reporters=0) when no place.stream.media.viewCount records have been observed yet, so consumers can render a count unconditionally."
        },
        "tracks": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "place.stream.media.track#trackView"
          }
        }
      }
    },
    "viewCountSummary": {
      "type": "object",
      "description": "Sums across every place.stream.media.viewCount record indexed for this video, regardless of reporter or window. The underlying records are window-bounded; the consumer sees a running total.",
      "required": ["count", "bytes", "durationMs", "reporters"],
      "properties": {
        "count": {
          "type": "integer",
          "minimum": 0,
          "description": "Sum of `count` across every indexed report."
        },
        "bytes": {
          "type": "integer",
          "minimum": 0,
          "description": "Sum of bytes transferred across every track row in every indexed report."
        },
        "durationMs": {
          "type": "integer",
          "minimum": 0,
          "description": "Sum of playback duration (ms) across every track row in every indexed report."
        },
        "reporters": {
          "type": "integer",
          "minimum": 0,
          "description": "Number of distinct streamplace nodes that have contributed at least one viewCount record for this video."
        }
      }
    }
  }
}
```
