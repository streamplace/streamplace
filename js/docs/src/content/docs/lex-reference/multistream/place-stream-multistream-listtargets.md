---
title: place.stream.multistream.listTargets
description: Reference for the place.stream.multistream.listTargets lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

List a range of targets for rebroadcasting a Streamplace stream.

**Parameters:**

| Name     | Type      | Req'd | Description                      | Constraints                           |
| -------- | --------- | ----- | -------------------------------- | ------------------------------------- |
| `limit`  | `integer` | ❌    | The number of targets to return. | Min: 1<br/>Max: 100<br/>Default: `50` |
| `cursor` | `string`  | ❌    |                                  |                                       |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                                                           | Req'd | Description | Constraints |
| --------- | -------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `targets` | Array of [`place.stream.multistream.defs#targetView`](/lex-reference/place-stream-multistream-defs#targetview) | ✅    |             |             |
| `cursor`  | `string`                                                                                                       | ❌    |             |             |

---

<a name="record"></a>

### `record`

**Type:** `object`

**Properties:**

| Name    | Type      | Req'd | Description | Constraints      |
| ------- | --------- | ----- | ----------- | ---------------- |
| `uri`   | `string`  | ✅    |             | Format: `at-uri` |
| `cid`   | `string`  | ✅    |             | Format: `cid`    |
| `value` | `unknown` | ✅    |             |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.multistream.listTargets",
  "defs": {
    "main": {
      "type": "query",
      "description": "List a range of targets for rebroadcasting a Streamplace stream.",
      "parameters": {
        "type": "params",
        "required": [],
        "properties": {
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 50,
            "description": "The number of targets to return."
          },
          "cursor": {
            "type": "string"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["targets"],
          "properties": {
            "targets": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.multistream.defs#targetView"
              }
            },
            "cursor": {
              "type": "string"
            }
          }
        }
      }
    },
    "record": {
      "type": "object",
      "required": ["uri", "cid", "value"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "value": {
          "type": "unknown"
        }
      }
    }
  }
}
```
