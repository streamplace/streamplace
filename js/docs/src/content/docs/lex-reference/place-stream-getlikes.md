---
title: place.stream.getLikes
description: Reference for the place.stream.getLikes lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get likes for a subject (video or comment).

**Parameters:**

| Name      | Type      | Req'd | Description                               | Constraints                           |
| --------- | --------- | ----- | ----------------------------------------- | ------------------------------------- |
| `subject` | `string`  | ✅    | AT-URI of the subject (video or comment). | Format: `at-uri`                      |
| `cursor`  | `string`  | ❌    | Optional pagination cursor.               |                                       |
| `limit`   | `integer` | ❌    | Maximum number of likes to return.        | Min: 1<br/>Max: 100<br/>Default: `50` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                              | Req'd | Description                             | Constraints      |
| --------- | --------------------------------- | ----- | --------------------------------------- | ---------------- |
| `subject` | `string`                          | ✅    |                                         | Format: `at-uri` |
| `count`   | `integer`                         | ✅    | Total number of likes for this subject. | Min: 0           |
| `cursor`  | `string`                          | ❌    |                                         |                  |
| `likes`   | Array of [`#likeView`](#likeview) | ❌    |                                         |                  |

---

<a name="likeview"></a>

### `likeView`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                             | Req'd | Description | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |             | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |             | Format: `cid`      |
| `author`    | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |             |                    |
| `record`    | `unknown`                                                                                                                                        | ✅    |             |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |             | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.getLikes",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get likes for a subject (video or comment).",
      "parameters": {
        "type": "params",
        "required": ["subject"],
        "properties": {
          "subject": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the subject (video or comment)."
          },
          "cursor": {
            "type": "string",
            "description": "Optional pagination cursor."
          },
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 50,
            "description": "Maximum number of likes to return."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["subject", "count"],
          "properties": {
            "subject": {
              "type": "string",
              "format": "at-uri"
            },
            "count": {
              "type": "integer",
              "minimum": 0,
              "description": "Total number of likes for this subject."
            },
            "cursor": {
              "type": "string"
            },
            "likes": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#likeView"
              }
            }
          }
        }
      }
    },
    "likeView": {
      "type": "object",
      "required": ["uri", "cid", "author", "record", "indexedAt"],
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
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        }
      }
    }
  }
}
```
