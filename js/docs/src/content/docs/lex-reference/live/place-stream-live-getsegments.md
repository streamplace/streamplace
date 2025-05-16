---
title: place.stream.live.getSegments
description: Reference for the place.stream.live.getSegments lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a list of livestream segments for a user

**Parameters:**

| Name      | Type      | Req'd | Description                               | Constraints                           |
| --------- | --------- | ----- | ----------------------------------------- | ------------------------------------- |
| `userDID` | `string`  | ✅    | The DID of the potentially-following user | Format: `did`                         |
| `limit`   | `integer` | ❌    |                                           | Min: 1<br/>Max: 100<br/>Default: `50` |
| `before`  | `string`  | ❌    |                                           | Format: `datetime`                    |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                                                                                           | Req'd | Description | Constraints |
| ---------- | ---------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `segments` | Array of [`place.stream.segment#segmentView`](/lex-reference/place-stream-segment#segmentview) | ❌    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.getSegments",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a list of livestream segments for a user",
      "parameters": {
        "type": "params",
        "required": ["userDID"],
        "properties": {
          "userDID": {
            "type": "string",
            "format": "did",
            "description": "The DID of the potentially-following user"
          },
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
            "segments": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.segment#segmentView"
              }
            }
          }
        }
      }
    }
  }
}
```
