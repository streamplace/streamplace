---
title: place.stream.vod.getComments
description: Reference for the place.stream.vod.getComments lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get comments for a place.stream.video record.

**Parameters:**

| Name     | Type      | Req'd | Description                              | Constraints                           |
| -------- | --------- | ----- | ---------------------------------------- | ------------------------------------- |
| `video`  | `string`  | ✅    | AT-URI of the place.stream.video record. | Format: `at-uri`                      |
| `cursor` | `string`  | ❌    | Optional pagination cursor.              |                                       |
| `limit`  | `integer` | ❌    | Maximum number of comments to return.    | Min: 1<br/>Max: 100<br/>Default: `50` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                                                                                             | Req'd | Description | Constraints |
| ---------- | ------------------------------------------------------------------------------------------------ | ----- | ----------- | ----------- |
| `cursor`   | `string`                                                                                         | ❌    |             |             |
| `comments` | Array of [`place.stream.vod.defs#commentView`](/lex-reference/place-stream-vod-defs#commentview) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.getComments",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get comments for a place.stream.video record.",
      "parameters": {
        "type": "params",
        "required": ["video"],
        "properties": {
          "video": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.video record."
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
            "description": "Maximum number of comments to return."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["comments"],
          "properties": {
            "cursor": {
              "type": "string"
            },
            "comments": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.vod.defs#commentView"
              }
            }
          }
        }
      }
    }
  }
}
```
