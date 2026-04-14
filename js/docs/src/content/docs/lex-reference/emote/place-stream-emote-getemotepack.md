---
title: place.stream.emote.getEmotePack
description: Reference for the place.stream.emote.getEmotePack lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get an emote pack and its items.

**Parameters:**

| Name  | Type     | Req'd | Description                                   | Constraints      |
| ----- | -------- | ----- | --------------------------------------------- | ---------------- |
| `uri` | `string` | ✅    | AT-URI of the place.stream.emote.pack record. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name   | Type                                                                                  | Req'd | Description | Constraints |
| ------ | ------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `pack` | [`place.stream.emote.defs#packView`](/lex-reference/place-stream-emote-defs#packview) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emote.getEmotePack",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get an emote pack and its items.",
      "parameters": {
        "type": "params",
        "required": ["uri"],
        "properties": {
          "uri": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.emote.pack record."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["pack"],
          "properties": {
            "pack": {
              "type": "ref",
              "ref": "place.stream.emote.defs#packView"
            }
          }
        }
      }
    }
  }
}
```
