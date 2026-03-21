---
title: place.stream.live.viewCount
description: Reference for the place.stream.live.viewCount lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Current view count for a livestream on a particular server.

**Record Key:** `any`

**Record Properties:**

| Name        | Type      | Req'd | Description                                      | Constraints        |
| ----------- | --------- | ----- | ------------------------------------------------ | ------------------ |
| `streamer`  | `string`  | ✅    | The DID of the streamer to teleport to.          | Format: `did`      |
| `server`    | `string`  | ✅    | The DID of the server to get the view count for. | Format: `did`      |
| `count`     | `integer` | ✅    | The current view count for the livestream.       |                    |
| `updatedAt` | `string`  | ❌    | The time the view count was last updated.        | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.viewCount",
  "defs": {
    "main": {
      "type": "record",
      "key": "any",
      "description": "Current view count for a livestream on a particular server.",
      "record": {
        "type": "object",
        "required": ["streamer", "server", "count"],
        "properties": {
          "streamer": {
            "type": "string",
            "format": "did",
            "description": "The DID of the streamer to teleport to."
          },
          "server": {
            "type": "string",
            "format": "did",
            "description": "The DID of the server to get the view count for."
          },
          "count": {
            "type": "integer",
            "description": "The current view count for the livestream."
          },
          "updatedAt": {
            "type": "string",
            "format": "datetime",
            "description": "The time the view count was last updated."
          }
        }
      }
    }
  }
}
```
