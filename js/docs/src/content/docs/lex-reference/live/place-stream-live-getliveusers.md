---
title: place.stream.live.getLiveUsers
description: Reference for the place.stream.live.getLiveUsers lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a list of livestream segments for a user

**Parameters:**

| Name     | Type      | Req'd | Description | Constraints                           |
| -------- | --------- | ----- | ----------- | ------------------------------------- |
| `limit`  | `integer` | ❌    |             | Min: 1<br/>Max: 100<br/>Default: `50` |
| `before` | `string`  | ❌    |             | Format: `datetime`                    |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                                                       | Req'd | Description | Constraints |
| --------- | ---------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `streams` | Array of [`place.stream.livestream#livestreamView`](/lex-reference/place-stream-livestream#livestreamview) | ❌    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.getLiveUsers",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a list of livestream segments for a user",
      "parameters": {
        "type": "params",
        "properties": {
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 50
          },
          "before": {
            "type": "string",
            "format": "datetime"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "properties": {
            "streams": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.livestream#livestreamView"
              }
            }
          }
        }
      }
    }
  }
}
```
