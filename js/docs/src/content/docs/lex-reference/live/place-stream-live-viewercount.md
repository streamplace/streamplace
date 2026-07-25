---
title: place.stream.live.viewerCount
description: Reference for the place.stream.live.viewerCount lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Current viewer count for a livestream on a particular server. Record keys are streamer_did::server_did by convention.

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
  "id": "place.stream.live.viewerCount",
  "defs": {
    "main": {
      "type": "record",
      "key": "any",
      "description": "Current viewer count for a livestream on a particular server. Record keys are streamer_did::server_did by convention.",
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
