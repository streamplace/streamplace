---
title: place.stream.media.getUploadStatus
description: Reference for the place.stream.media.getUploadStatus lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get the processing status of a previously created upload. Only accessible by the DID that created the upload.

**Parameters:**

| Name       | Type     | Req'd | Description                                                | Constraints |
| ---------- | -------- | ----- | ---------------------------------------------------------- | ----------- |
| `uploadId` | `string` | ✅    | The upload ID returned by place.stream.media.createUpload. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type                              | Req'd | Description                                                                          | Constraints                                            |
| ------------ | --------------------------------- | ----- | ------------------------------------------------------------------------------------ | ------------------------------------------------------ |
| `status`     | `string`                          | ✅    | Current processing status of the upload.                                             | Known Values: `pending`, `processing`, `done`, `error` |
| `progress`   | `integer`                         | ❌    | Processing progress percentage (0-100). Only meaningful when status is 'processing'. | Min: 0<br/>Max: 100                                    |
| `tracks`     | Array of [`#trackRef`](#trackref) | ❌    | Published track records. Present when status is 'done'.                              |                                                        |
| `durationMs` | `integer`                         | ❌    | Duration of the processed video in milliseconds. Present when status is 'done'.      |                                                        |
| `error`      | `string`                          | ❌    | Error message. Present when status is 'error'.                                       |                                                        |

**Possible Errors:**

- `NotFound`: No upload exists with the given ID for the authenticated user.

---

<a name="trackref"></a>

### `trackRef`

**Type:** `object`

**Properties:**

| Name  | Type     | Req'd | Description | Constraints      |
| ----- | -------- | ----- | ----------- | ---------------- |
| `uri` | `string` | ✅    |             | Format: `at-uri` |
| `cid` | `string` | ✅    |             |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.getUploadStatus",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get the processing status of a previously created upload. Only accessible by the DID that created the upload.",
      "parameters": {
        "type": "params",
        "required": ["uploadId"],
        "properties": {
          "uploadId": {
            "type": "string",
            "description": "The upload ID returned by place.stream.media.createUpload."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["status"],
          "properties": {
            "status": {
              "type": "string",
              "knownValues": ["pending", "processing", "done", "error"],
              "description": "Current processing status of the upload."
            },
            "progress": {
              "type": "integer",
              "minimum": 0,
              "maximum": 100,
              "description": "Processing progress percentage (0-100). Only meaningful when status is 'processing'."
            },
            "tracks": {
              "type": "array",
              "description": "Published track records. Present when status is 'done'.",
              "items": {
                "type": "ref",
                "ref": "#trackRef"
              }
            },
            "durationMs": {
              "type": "integer",
              "description": "Duration of the processed video in milliseconds. Present when status is 'done'."
            },
            "error": {
              "type": "string",
              "description": "Error message. Present when status is 'error'."
            }
          }
        }
      },
      "errors": [
        {
          "name": "NotFound",
          "description": "No upload exists with the given ID for the authenticated user."
        }
      ]
    },
    "trackRef": {
      "type": "object",
      "required": ["uri", "cid"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string"
        }
      }
    }
  }
}
```
