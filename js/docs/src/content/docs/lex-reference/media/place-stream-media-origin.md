---
title: place.stream.media.origin
description: Reference for the place.stream.media.origin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

An attestation that a MUXL blob is available for download from the publishing node, retrievable via XRPC (com.atproto.repo.getBlob and similar). Published by the Streamplace node that hosts the blob, not by the user who owns the underlying video. The rkey is conventionally the blob's BDASL CID — atproto lexicon doesn't yet have a literal-rkey syntax so we settle for `key: any` and rely on the convention.

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
      "description": "An attestation that a MUXL blob is available for download from the publishing node, retrievable via XRPC (com.atproto.repo.getBlob and similar). Published by the Streamplace node that hosts the blob, not by the user who owns the underlying video. The rkey is conventionally the blob's BDASL CID — atproto lexicon doesn't yet have a literal-rkey syntax so we settle for `key: any` and rely on the convention.",
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
