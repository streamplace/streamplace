---
title: place.stream.live.stopLivestream
description: Reference for the place.stream.live.stopLivestream lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Stop your current livestream, updating your current place.stream.livestream record and ceasing the flow of video.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_
**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                                   | Constraints   |
| ----- | -------- | ----- | --------------------------------------------- | ------------- |
| `uri` | `string` | ✅    | The URI of the stopped livestream record.     | Format: `uri` |
| `cid` | `string` | ✅    | The new CID of the stopped livestream record. | Format: `cid` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.stopLivestream",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Stop your current livestream, updating your current place.stream.livestream record and ceasing the flow of video.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": [],
          "properties": {}
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri", "cid"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "uri",
              "description": "The URI of the stopped livestream record."
            },
            "cid": {
              "type": "string",
              "format": "cid",
              "description": "The new CID of the stopped livestream record."
            }
          }
        }
      }
    }
  }
}
```
