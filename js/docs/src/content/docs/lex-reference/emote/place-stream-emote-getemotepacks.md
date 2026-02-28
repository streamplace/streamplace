---
title: place.stream.emote.getEmotePacks
description: Reference for the place.stream.emote.getEmotePacks lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get all emote packs available on this server.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name    | Type                                                                                           | Req'd | Description | Constraints |
| ------- | ---------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `packs` | Array of [`place.stream.emote.defs#packView`](/lex-reference/place-stream-emote-defs#packview) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emote.getEmotePacks",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get all emote packs available on this server.",
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["packs"],
          "properties": {
            "packs": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.emote.defs#packView"
              }
            }
          }
        }
      }
    }
  }
}
```
