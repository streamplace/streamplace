---
title: place.stream.playback.getVideoBlob
description: Reference for the place.stream.playback.getVideoBlob lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Fetch bytes from a video's MUXL archive. Supports HTTP Range headers for byte range requests. The node resolves byte ranges against the virtual archive file, serving data from mempool or archival storage as appropriate.

**Parameters:**

| Name   | Type     | Req'd | Description                                                                                                                            | Constraints   |
| ------ | -------- | ----- | -------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| `did`  | `string` | ✅    | DID of the video creator.                                                                                                              | Format: `did` |
| `rkey` | `string` | ✅    | Record key of the place.stream.video record.                                                                                           |               |
| `cid`  | `string` | ❌    | Optional CID of a specific segment blob. If provided, returns the segment data directly instead of resolving from the virtual archive. | Format: `cid` |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `VideoNotFound`: No video record found for this DID and rkey.
- `BlobNotFound`: The requested CID or byte range is not available.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.playback.getVideoBlob",
  "defs": {
    "main": {
      "type": "query",
      "description": "Fetch bytes from a video's MUXL archive. Supports HTTP Range headers for byte range requests. The node resolves byte ranges against the virtual archive file, serving data from mempool or archival storage as appropriate.",
      "parameters": {
        "type": "params",
        "required": ["did", "rkey"],
        "properties": {
          "did": {
            "type": "string",
            "format": "did",
            "description": "DID of the video creator."
          },
          "rkey": {
            "type": "string",
            "description": "Record key of the place.stream.video record."
          },
          "cid": {
            "type": "string",
            "format": "cid",
            "description": "Optional CID of a specific segment blob. If provided, returns the segment data directly instead of resolving from the virtual archive."
          }
        }
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "VideoNotFound",
          "description": "No video record found for this DID and rkey."
        },
        {
          "name": "BlobNotFound",
          "description": "The requested CID or byte range is not available."
        }
      ]
    }
  }
}
```
