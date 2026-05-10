---
title: place.stream.media.origin
description: Reference for the place.stream.media.origin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A record indicating that a MUXL blob is available for download somewhere.

**Properties:**

| Name       | Type      | Req'd | Description                                                   | Constraints |
| ---------- | --------- | ----- | ------------------------------------------------------------- | ----------- |
| `blob`     | `string`  | ✅    | BLAKE-3 content hash (BDASL CID) of the source video segment. |             |
| `size`     | `integer` | ✅    | Size of the file in bytes.                                    |             |
| `mimeType` | `string`  | ✅    | MIME type of the file (e.g. video/mp4).                       |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.origin",
  "defs": {
    "main": {
      "type": "object",
      "description": "A record indicating that a MUXL blob is available for download somewhere.",
      "required": ["blob", "size", "mimeType"],
      "properties": {
        "blob": {
          "type": "string",
          "description": "BLAKE-3 content hash (BDASL CID) of the source video segment."
        },
        "size": {
          "type": "integer",
          "description": "Size of the file in bytes."
        },
        "mimeType": {
          "type": "string",
          "description": "MIME type of the file (e.g. video/mp4)."
        }
      }
    }
  }
}
```
