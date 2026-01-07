---
title: place.stream.live.subscribeSegments
description: Reference for the place.stream.live.subscribeSegments lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `subscription`

Subscribe to a stream's new segments as they come in!

**Parameters:**

| Name       | Type     | Req'd | Description                             | Constraints |
| ---------- | -------- | ----- | --------------------------------------- | ----------- |
| `streamer` | `string` | ✅    | The DID of the streamer to subscribe to |             |

**Message:**

- **Schema:**

**Schema Type:** Union of:<br/>&nbsp;&nbsp;[`#segment`](#segment)

---

<a name="segment"></a>

### `segment`

**Type:** `bytes`

MP4 file of a user's signed livestream segment

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.subscribeSegments",
  "defs": {
    "main": {
      "type": "subscription",
      "description": "Subscribe to a stream's new segments as they come in!",
      "parameters": {
        "type": "params",
        "required": ["streamer"],
        "properties": {
          "streamer": {
            "type": "string",
            "description": "The DID of the streamer to subscribe to"
          }
        }
      },
      "message": {
        "schema": {
          "type": "union",
          "refs": ["#segment"]
        }
      },
      "errors": []
    },
    "segment": {
      "type": "bytes",
      "description": "MP4 file of a user's signed livestream segment"
    }
  }
}
```
