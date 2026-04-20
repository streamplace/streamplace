---
title: place.stream.playback.getPlaybackServer
description: Reference for the place.stream.playback.getPlaybackServer lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get available playback servers for a livestream.

**Parameters:**

| Name     | Type     | Req'd | Description                                          | Constraints |
| -------- | -------- | ----- | ---------------------------------------------------- | ----------- |
| `stream` | `string` | ✅    | Identifier of the stream to get playback servers for |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type              | Req'd | Description                                 | Constraints |
| --------- | ----------------- | ----- | ------------------------------------------- | ----------- |
| `servers` | Array of `string` | ✅    | List of available playback server addresses |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getPlaybackServer",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get available playback servers for a livestream.",
      "parameters": {
        "type": "params",
        "required": ["stream"],
        "properties": {
          "stream": {
            "type": "string",
            "description": "Identifier of the stream to get playback servers for"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["servers"],
          "properties": {
            "servers": {
              "type": "array",
              "items": {
                "type": "string"
              },
              "description": "List of available playback server addresses"
            }
          }
        }
      },
      "errors": []
    }
  }
}
```
