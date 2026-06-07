---
title: place.stream.media.finalizeLivestream
description: Reference for the place.stream.media.finalizeLivestream lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Turn a finished livestream into a VOD. The server concatenates the MUXL segments it recorded for the livestream into a single content blob, derives the playback sidecars, and publishes the place.stream.media.track records — the same end state as a finished upload. Returns an uploadId the client polls with place.stream.media.getUploadStatus and then publishes with place.stream.media.publishVideo, exactly as for a resumable upload.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type     | Req'd | Description                                                                                                 | Constraints      |
| ------------ | -------- | ----- | ----------------------------------------------------------------------------------------------------------- | ---------------- |
| `livestream` | `string` | ✅    | AT-URI of the place.stream.livestream record to finalize into a VOD. Must belong to the authenticated user. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                                                                                                                                                     | Constraints |
| ---------- | -------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `uploadId` | `string` | ✅    | Identifier for the finalize job. Poll place.stream.media.getUploadStatus with it; once status is 'done', create the video with place.stream.media.publishVideo. |             |

**Possible Errors:**

- `LivestreamNotFound`: No livestream with the given URI is known, or it does not belong to the authenticated user.
- `NoRecording`: The livestream has no recorded MUXL segments to finalize (recording was not enabled, or none completed).

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.finalizeLivestream",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Turn a finished livestream into a VOD. The server concatenates the MUXL segments it recorded for the livestream into a single content blob, derives the playback sidecars, and publishes the place.stream.media.track records — the same end state as a finished upload. Returns an uploadId the client polls with place.stream.media.getUploadStatus and then publishes with place.stream.media.publishVideo, exactly as for a resumable upload.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["livestream"],
          "properties": {
            "livestream": {
              "type": "string",
              "format": "at-uri",
              "description": "AT-URI of the place.stream.livestream record to finalize into a VOD. Must belong to the authenticated user."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uploadId"],
          "properties": {
            "uploadId": {
              "type": "string",
              "description": "Identifier for the finalize job. Poll place.stream.media.getUploadStatus with it; once status is 'done', create the video with place.stream.media.publishVideo."
            }
          }
        }
      },
      "errors": [
        {
          "name": "LivestreamNotFound",
          "description": "No livestream with the given URI is known, or it does not belong to the authenticated user."
        },
        {
          "name": "NoRecording",
          "description": "The livestream has no recorded MUXL segments to finalize (recording was not enabled, or none completed)."
        }
      ]
    }
  }
}
```
