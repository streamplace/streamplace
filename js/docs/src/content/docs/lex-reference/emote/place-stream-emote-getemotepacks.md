---
title: place.stream.emote.getEmotePacks
description: Reference for the place.stream.emote.getEmotePacks lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get emote packs available to the viewer in a specific stream's chat.

**Parameters:**

| Name       | Type     | Req'd | Description                                                          | Constraints   |
| ---------- | -------- | ----- | -------------------------------------------------------------------- | ------------- |
| `streamer` | `string` | ✅    | The DID of the streamer whose chat context to fetch emote packs for. | Format: `did` |

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
      "description": "Get emote packs available to the viewer in a specific stream's chat.",
      "parameters": {
        "type": "params",
        "required": ["streamer"],
        "properties": {
          "streamer": {
            "type": "string",
            "format": "did",
            "description": "The DID of the streamer whose chat context to fetch emote packs for."
          }
        }
      },
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
