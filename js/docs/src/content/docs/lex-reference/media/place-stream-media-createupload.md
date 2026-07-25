---
title: place.stream.media.createUpload
description: Reference for the place.stream.media.createUpload lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Start a resumable upload of arbitrary media content. Returns a TUS upload URL and a short-lived bearer token used to authenticate subsequent chunk requests (no DPoP required on chunks). The DID of the authenticated user is recorded against the upload; on completion a VOD processing task is enqueued.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type      | Req'd | Description                                                                                                                                                                                                                                                                                               | Constraints |
| ---------- | --------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `size`     | `integer` | ✅    | Total size of the upload in bytes.                                                                                                                                                                                                                                                                        |             |
| `mimeType` | `string`  | ✅    | MIME type of the content being uploaded (e.g. video/mp4).                                                                                                                                                                                                                                                 |             |
| `filename` | `string`  | ❌    | Optional filename hint to attach as upload metadata.                                                                                                                                                                                                                                                      |             |
| `draftUri` | `string`  | ❌    | Optional ats:// URI of a draft VOD created via place.stream.vod.createDraft. When supplied, the upload's processing fills that draft (rather than creating a new one), so the user can edit metadata while the upload runs and re-upload if it fails. The draft's origin_upload_id is set to this upload. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type     | Req'd | Description                                                                                             | Constraints        |
| ------------- | -------- | ----- | ------------------------------------------------------------------------------------------------------- | ------------------ |
| `uploadId`    | `string` | ✅    | Server-side identifier for this upload.                                                                 |                    |
| `uploadUrl`   | `string` | ✅    | Absolute URL to PATCH/HEAD per the TUS protocol.                                                        | Format: `uri`      |
| `uploadToken` | `string` | ✅    | Short-lived bearer token bound to this upload. Send as 'Authorization: Bearer <token>' on TUS requests. |                    |
| `expiresAt`   | `string` | ✅    | When the upload token expires.                                                                          | Format: `datetime` |

**Possible Errors:**

- `UploadTooLarge`: The requested upload exceeds the server's maximum allowed size.
- `UnsupportedMimeType`: The server does not accept this MIME type for upload.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.createUpload",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Start a resumable upload of arbitrary media content. Returns a TUS upload URL and a short-lived bearer token used to authenticate subsequent chunk requests (no DPoP required on chunks). The DID of the authenticated user is recorded against the upload; on completion a VOD processing task is enqueued.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["size", "mimeType"],
          "properties": {
            "size": {
              "type": "integer",
              "description": "Total size of the upload in bytes."
            },
            "mimeType": {
              "type": "string",
              "description": "MIME type of the content being uploaded (e.g. video/mp4)."
            },
            "filename": {
              "type": "string",
              "description": "Optional filename hint to attach as upload metadata."
            },
            "draftUri": {
              "type": "string",
              "description": "Optional ats:// URI of a draft VOD created via place.stream.vod.createDraft. When supplied, the upload's processing fills that draft (rather than creating a new one), so the user can edit metadata while the upload runs and re-upload if it fails. The draft's origin_upload_id is set to this upload."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uploadUrl", "uploadToken", "expiresAt", "uploadId"],
          "properties": {
            "uploadId": {
              "type": "string",
              "description": "Server-side identifier for this upload."
            },
            "uploadUrl": {
              "type": "string",
              "format": "uri",
              "description": "Absolute URL to PATCH/HEAD per the TUS protocol."
            },
            "uploadToken": {
              "type": "string",
              "description": "Short-lived bearer token bound to this upload. Send as 'Authorization: Bearer <token>' on TUS requests."
            },
            "expiresAt": {
              "type": "string",
              "format": "datetime",
              "description": "When the upload token expires."
            }
          }
        }
      },
      "errors": [
        {
          "name": "UploadTooLarge",
          "description": "The requested upload exceeds the server's maximum allowed size."
        },
        {
          "name": "UnsupportedMimeType",
          "description": "The server does not accept this MIME type for upload."
        }
      ]
    }
  }
}
```
