---
title: place.stream.media.origin
description: Reference for the place.stream.media.origin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

The publishing station attests that it serves this public MUXL blob. The record key is the blob's BDASL CID. Resolve the station DID's #streamplace service and call place.stream.playback.getVideoBlob there. The #atproto_pds service hosts the attestation, not the media. Independent stations may attest to the same blob.

**Record Key:** `any`

**Record Properties:**

| Name       | Type      | Req'd | Description                                   | Constraints |
| ---------- | --------- | ----- | --------------------------------------------- | ----------- |
| `blob`     | `string`  | ✅    | BLAKE-3 content hash (BDASL CID) of the blob. |             |
| `size`     | `integer` | ✅    | Size of the blob in bytes.                    |             |
| `mimeType` | `string`  | ✅    | MIME type of the blob (e.g. video/mp4).       |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.origin",
  "defs": {
    "main": {
      "type": "record",
      "description": "The publishing station attests that it serves this public MUXL blob. The record key is the blob's BDASL CID. Resolve the station DID's #streamplace service and call place.stream.playback.getVideoBlob there. The #atproto_pds service hosts the attestation, not the media. Independent stations may attest to the same blob.",
      "key": "any",
      "record": {
        "type": "object",
        "required": ["blob", "size", "mimeType"],
        "properties": {
          "blob": {
            "type": "string",
            "description": "BLAKE-3 content hash (BDASL CID) of the blob."
          },
          "size": {
            "type": "integer",
            "description": "Size of the blob in bytes."
          },
          "mimeType": {
            "type": "string",
            "description": "MIME type of the blob (e.g. video/mp4)."
          }
        }
      }
    }
  }
}
```
