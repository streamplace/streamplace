---
title: place.stream.playback.getPlaybackSession
description: Reference for the place.stream.playback.getPlaybackSession lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Mint the caller's own playback session for their live stream. Every playback URL carries a signed session as `sid`; the anonymous one a playlist hands out opens published streams only, while the owner session this returns also opens the caller's unpublished (pre-live) stream to their own player. Requires an authenticated session; the result is bound to the caller's DID. Pass it as `sid` on getLivePlaylist.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name        | Type     | Req'd | Description                                                                            | Constraints        |
| ----------- | -------- | ----- | -------------------------------------------------------------------------------------- | ------------------ |
| `sid`       | `string` | ✅    | The signed session to pass as the `sid` query parameter.                               |                    |
| `expiresAt` | `string` | ✅    | When the session stops opening the stream unless renewed; fetch a new one before then. | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getPlaybackSession",
  "defs": {
    "main": {
      "type": "query",
      "description": "Mint the caller's own playback session for their live stream. Every playback URL carries a signed session as `sid`; the anonymous one a playlist hands out opens published streams only, while the owner session this returns also opens the caller's unpublished (pre-live) stream to their own player. Requires an authenticated session; the result is bound to the caller's DID. Pass it as `sid` on getLivePlaylist.",
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["sid", "expiresAt"],
          "properties": {
            "sid": {
              "type": "string",
              "description": "The signed session to pass as the `sid` query parameter."
            },
            "expiresAt": {
              "type": "string",
              "format": "datetime",
              "description": "When the session stops opening the stream unless renewed; fetch a new one before then."
            }
          }
        }
      }
    }
  }
}
```
