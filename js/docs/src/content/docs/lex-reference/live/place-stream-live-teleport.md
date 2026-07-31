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

| Name              | Type                                                                                                                                   | Req'd | Description                                                                                                                                                                                                                                                                                               | Constraints            |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------- |
| `streamer`        | `string`                                                                                                                               | ✅    | The DID of the streamer to teleport to.                                                                                                                                                                                                                                                                   | Format: `did`          |
| `startsAt`        | `string`                                                                                                                               | ✅    | The time the teleport becomes active.                                                                                                                                                                                                                                                                     | Format: `datetime`     |
| `livestream`      | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | The source livestream this teleport is sending viewers away from. When the teleport fires, this is the livestream that gets ended (the same update place.stream.live.stopLivestream performs), so the source streamer returns to pre-live. Teleports without an origin livestream are treated as a no-op. |                        |
| `durationSeconds` | `integer`                                                                                                                              | ❌    | The time limit in seconds for the teleport. If not set, the teleport is permanent. Must be at least 60 seconds, and no more than 32,400 seconds (9 hours).                                                                                                                                                | Min: 60<br/>Max: 32400 |

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
          "livestream": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The source livestream this teleport is sending viewers away from. When the teleport fires, this is the livestream that gets ended (the same update place.stream.live.stopLivestream performs), so the source streamer returns to pre-live. Teleports without an origin livestream are treated as a no-op."
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
