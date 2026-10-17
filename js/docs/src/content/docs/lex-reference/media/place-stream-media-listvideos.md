---
title: place.stream.media.listVideos
description: Reference for the place.stream.media.listVideos lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

A streamer's place.stream.video records (read from their PDS) and this node's uploads for them: everything updateVideo, deleteVideo and publishVideo take. The caller must be the streamer or hold livestream.manage from them.

**Parameters:**

| Name   | Type     | Req'd | Description         | Constraints   |
| ------ | -------- | ----- | ------------------- | ------------- |
| `repo` | `string` | ✅    | The streamer's DID. | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                          | Req'd | Description | Constraints   |
| --------- | ----------------------------- | ----- | ----------- | ------------- |
| `repoDID` | `string`                      | ✅    |             | Format: `did` |
| `videos`  | Array of [`#video`](#video)   | ✅    |             |               |
| `uploads` | Array of [`#upload`](#upload) | ✅    |             |               |

**Possible Errors:**

- `NotPermitted`: The caller is neither the streamer nor a moderator with livestream.manage from them.

---

<a name="video"></a>

### `video`

**Type:** `object`

**Properties:**

| Name          | Type              | Req'd | Description                                       | Constraints      |
| ------------- | ----------------- | ----- | ------------------------------------------------- | ---------------- |
| `uri`         | `string`          | ✅    |                                                   | Format: `at-uri` |
| `cid`         | `string`          | ✅    |                                                   | Format: `cid`    |
| `title`       | `string`          | ✅    |                                                   |                  |
| `durationMs`  | `integer`         | ❌    |                                                   |                  |
| `createdAt`   | `string`          | ❌    |                                                   |                  |
| `tracks`      | Array of `string` | ✅    | The track records the video's source points at.   |                  |
| `connections` | Array of `string` | ❌    | The livestream records the video is connected to. |                  |
| `thumb`       | `boolean`         | ❌    | Whether the record carries a thumbnail.           |                  |

---

<a name="upload"></a>

### `upload`

**Type:** `object`

**Properties:**

| Name         | Type      | Req'd | Description                                                    | Constraints        |
| ------------ | --------- | ----- | -------------------------------------------------------------- | ------------------ |
| `uploadId`   | `string`  | ✅    |                                                                |                    |
| `status`     | `string`  | ✅    | Processing status: pending, processing, done or error.         |                    |
| `error`      | `string`  | ❌    |                                                                |                    |
| `backend`    | `string`  | ✅    |                                                                |                    |
| `location`   | `string`  | ❌    | For a finalized recording, the livestream record it came from. |                    |
| `contentCid` | `string`  | ❌    |                                                                |                    |
| `durationMs` | `integer` | ❌    |                                                                |                    |
| `blobSize`   | `integer` | ❌    |                                                                |                    |
| `tracks`     | `integer` | ❌    | How many track records the upload has published.               |                    |
| `createdAt`  | `string`  | ❌    |                                                                | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.listVideos",
  "defs": {
    "main": {
      "type": "query",
      "description": "A streamer's place.stream.video records (read from their PDS) and this node's uploads for them: everything updateVideo, deleteVideo and publishVideo take. The caller must be the streamer or hold livestream.manage from them.",
      "parameters": {
        "type": "params",
        "required": ["repo"],
        "properties": {
          "repo": {
            "type": "string",
            "format": "did",
            "description": "The streamer's DID."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["repoDID", "videos", "uploads"],
          "properties": {
            "repoDID": {
              "type": "string",
              "format": "did"
            },
            "videos": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#video"
              }
            },
            "uploads": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#upload"
              }
            }
          }
        }
      },
      "errors": [
        {
          "name": "NotPermitted",
          "description": "The caller is neither the streamer nor a moderator with livestream.manage from them."
        }
      ]
    },
    "video": {
      "type": "object",
      "required": ["uri", "cid", "title", "tracks"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "title": {
          "type": "string"
        },
        "durationMs": {
          "type": "integer"
        },
        "createdAt": {
          "type": "string"
        },
        "tracks": {
          "type": "array",
          "items": {
            "type": "string",
            "format": "at-uri"
          },
          "description": "The track records the video's source points at."
        },
        "connections": {
          "type": "array",
          "items": {
            "type": "string",
            "format": "at-uri"
          },
          "description": "The livestream records the video is connected to."
        },
        "thumb": {
          "type": "boolean",
          "description": "Whether the record carries a thumbnail."
        }
      }
    },
    "upload": {
      "type": "object",
      "required": ["uploadId", "status", "backend"],
      "properties": {
        "uploadId": {
          "type": "string"
        },
        "status": {
          "type": "string",
          "description": "Processing status: pending, processing, done or error."
        },
        "error": {
          "type": "string"
        },
        "backend": {
          "type": "string"
        },
        "location": {
          "type": "string",
          "description": "For a finalized recording, the livestream record it came from."
        },
        "contentCid": {
          "type": "string"
        },
        "durationMs": {
          "type": "integer"
        },
        "blobSize": {
          "type": "integer"
        },
        "tracks": {
          "type": "integer",
          "description": "How many track records the upload has published."
        },
        "createdAt": {
          "type": "string",
          "format": "datetime"
        }
      }
    }
  }
}
```
