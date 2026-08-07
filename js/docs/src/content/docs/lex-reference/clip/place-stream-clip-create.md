---
title: place.stream.clip.create
description: Reference for the place.stream.clip.create lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create an ephemeral clip from a streamer's live broadcast. Grabs the last N milliseconds from the streamer's moderation buffer, muxes it into a temporary MP4, and creates a VOD draft pre-filled with the muxed content. The clipper has 10 minutes to edit and publish before the ephemeral file is deleted. Requires the streamer to have clipping enabled (livestreamClipping setting). Rate-limited per viewer and per stream.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type      | Req'd | Description                                                                                                                       | Constraints                                    |
| ------------ | --------- | ----- | --------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| `streamer`   | `string`  | ✅    | The DID of the streamer whose live broadcast is being clipped.                                                                    | Format: `did`                                  |
| `durationMs` | `integer` | ❌    | Duration of content to grab from the buffer, in milliseconds. Defaults to 60 seconds. Maximum is the buffer window (120 seconds). | Min: 1000<br/>Max: 120000<br/>Default: `60000` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type      | Req'd | Description                                                                                                         | Constraints        |
| ------------ | --------- | ----- | ------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `clipId`     | `string`  | ✅    | ID of the created ephemeral clip. Pass to place.stream.clip.publish within 10 minutes.                              |                    |
| `previewUrl` | `string`  | ✅    | URL to the ephemeral MP4 for preview in the clip editor. Valid until expiresAt.                                     |                    |
| `expiresAt`  | `string`  | ✅    | When the ephemeral clip file and draft will be deleted. Always 10 minutes from creation.                            | Format: `datetime` |
| `durationMs` | `integer` | ✅    | Actual duration of the muxed content, in milliseconds. May be less than requested if the buffer had fewer segments. |                    |

**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `ClippingDisabled`: The streamer has disabled clipping for their broadcasts.
- `RateLimited`: The viewer or stream has exceeded the clip rate limit.
- `NotLive`: The streamer is not currently broadcasting.
- `NoContent`: No segments were available in the moderation buffer to clip.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.clip.create",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create an ephemeral clip from a streamer's live broadcast. Grabs the last N milliseconds from the streamer's moderation buffer, muxes it into a temporary MP4, and creates a VOD draft pre-filled with the muxed content. The clipper has 10 minutes to edit and publish before the ephemeral file is deleted. Requires the streamer to have clipping enabled (livestreamClipping setting). Rate-limited per viewer and per stream.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer whose live broadcast is being clipped."
            },
            "durationMs": {
              "type": "integer",
              "default": 60000,
              "minimum": 1000,
              "maximum": 120000,
              "description": "Duration of content to grab from the buffer, in milliseconds. Defaults to 60 seconds. Maximum is the buffer window (120 seconds)."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["clipId", "previewUrl", "expiresAt", "durationMs"],
          "properties": {
            "clipId": {
              "type": "string",
              "description": "ID of the created ephemeral clip. Pass to place.stream.clip.publish within 10 minutes."
            },
            "previewUrl": {
              "type": "string",
              "description": "URL to the ephemeral MP4 for preview in the clip editor. Valid until expiresAt."
            },
            "expiresAt": {
              "type": "string",
              "format": "datetime",
              "description": "When the ephemeral clip file and draft will be deleted. Always 10 minutes from creation."
            },
            "durationMs": {
              "type": "integer",
              "description": "Actual duration of the muxed content, in milliseconds. May be less than requested if the buffer had fewer segments."
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The request lacks valid authentication credentials."
        },
        {
          "name": "ClippingDisabled",
          "description": "The streamer has disabled clipping for their broadcasts."
        },
        {
          "name": "RateLimited",
          "description": "The viewer or stream has exceeded the clip rate limit."
        },
        {
          "name": "NotLive",
          "description": "The streamer is not currently broadcasting."
        },
        {
          "name": "NoContent",
          "description": "No segments were available in the moderation buffer to clip."
        }
      ]
    }
  }
}
```
