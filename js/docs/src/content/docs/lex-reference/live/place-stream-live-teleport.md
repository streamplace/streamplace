---
title: place.stream.live.teleport
description: Reference for the place.stream.live.teleport lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record defining a 'teleport', that is active during a certain time.

**Record Key:** `tid`

**Record Properties:**

| Name              | Type      | Req'd | Description                                                                                                                                                | Constraints            |
| ----------------- | --------- | ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------- |
| `streamer`        | `string`  | ✅    | The DID of the streamer to teleport to.                                                                                                                    | Format: `did`          |
| `startsAt`        | `string`  | ✅    | The time the teleport becomes active.                                                                                                                      | Format: `datetime`     |
| `durationSeconds` | `integer` | ❌    | The time limit in seconds for the teleport. If not set, the teleport is permanent. Must be at least 60 seconds, and no more than 32,400 seconds (9 hours). | Min: 60<br/>Max: 32400 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.teleport",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record defining a 'teleport', that is active during a certain time.",
      "record": {
        "type": "object",
        "required": ["streamer", "startsAt"],
        "properties": {
          "streamer": {
            "type": "string",
            "format": "did",
            "description": "The DID of the streamer to teleport to."
          },
          "startsAt": {
            "type": "string",
            "format": "datetime",
            "description": "The time the teleport becomes active."
          },
          "durationSeconds": {
            "type": "integer",
            "description": "The time limit in seconds for the teleport. If not set, the teleport is permanent. Must be at least 60 seconds, and no more than 32,400 seconds (9 hours).",
            "minimum": 60,
            "maximum": 32400
          }
        }
      }
    }
  }
}
```
