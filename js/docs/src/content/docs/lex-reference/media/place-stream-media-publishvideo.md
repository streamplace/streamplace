---
title: place.stream.media.publishVideo
description: Reference for the place.stream.media.publishVideo lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Publish a place.stream.video record for a finished upload, server-side. The caller supplies the record it would otherwise putRecord itself; the server overrides the fields it is authoritative about (source tracks and durationMs, taken from the processed upload) and, if the record carries no thumb, generates one from the video and attaches it. The record is written to the authenticated user's repo.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                                                      | Req'd | Description                                                                                                                                                                               | Constraints |
| ---------- | --------------------------------------------------------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `uploadId` | `string`                                                  | ✅    | The upload ID returned by place.stream.media.createUpload. Its processing must be complete (status 'done').                                                                               |             |
| `record`   | [`place.stream.video`](/lex-reference/place-stream-video) | ✅    | A place.stream.video record. The server overrides `source` and `durationMs` from the processed upload, and fills in `thumb` with a generated thumbnail when the supplied record has none. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                                      | Constraints      |
| ----- | -------- | ----- | ------------------------------------------------ | ---------------- |
| `uri` | `string` | ✅    | AT-URI of the created place.stream.video record. | Format: `at-uri` |
| `cid` | `string` | ✅    | CID of the created place.stream.video record.    | Format: `cid`    |

**Possible Errors:**

- `UploadNotFound`: No upload with the given ID belongs to the authenticated user.
- `UploadNotReady`: The upload has not finished processing, so its tracks aren't available yet.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.publishVideo",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Publish a place.stream.video record for a finished upload, server-side. The caller supplies the record it would otherwise putRecord itself; the server overrides the fields it is authoritative about (source tracks and durationMs, taken from the processed upload) and, if the record carries no thumb, generates one from the video and attaches it. The record is written to the authenticated user's repo.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uploadId", "record"],
          "properties": {
            "uploadId": {
              "type": "string",
              "description": "The upload ID returned by place.stream.media.createUpload. Its processing must be complete (status 'done')."
            },
            "record": {
              "type": "ref",
              "ref": "place.stream.video",
              "description": "A place.stream.video record. The server overrides `source` and `durationMs` from the processed upload, and fills in `thumb` with a generated thumbnail when the supplied record has none."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri", "cid"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri",
              "description": "AT-URI of the created place.stream.video record."
            },
            "cid": {
              "type": "string",
              "format": "cid",
              "description": "CID of the created place.stream.video record."
            }
          }
        }
      },
      "errors": [
        {
          "name": "UploadNotFound",
          "description": "No upload with the given ID belongs to the authenticated user."
        },
        {
          "name": "UploadNotReady",
          "description": "The upload has not finished processing, so its tracks aren't available yet."
        }
      ]
    }
  }
}
```
