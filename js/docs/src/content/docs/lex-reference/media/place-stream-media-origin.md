---
title: place.stream.media.origin
description: Reference for the place.stream.media.origin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A MUXL blob in one of the MUXL-supported formats.

**Properties:**

| Name       | Type      | Req'd | Description                                      | Constraints |
| ---------- | --------- | ----- | ------------------------------------------------ | ----------- |
| `ref`      | `string`  | ✅    | BLAKE-3 content hash (BDASL CID) of the archive. |             |
| `muxlType` | `string`  | ✅    | MUXL type of the archive (mp4, fmp4).            |             |
| `size`     | `integer` | ✅    | Size of the file in bytes.                       |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.origin",
  "defs": {
    "main": {
      "type": "object",
      "description": "A MUXL blob in one of the MUXL-supported formats.",
      "required": ["ref", "muxlType", "size"],
      "properties": {
        "ref": {
          "type": "string",
          "description": "BLAKE-3 content hash (BDASL CID) of the archive."
        },
        "muxlType": {
          "type": "string",
          "description": "MUXL type of the archive (mp4, fmp4)."
        },
        "size": {
          "type": "integer",
          "description": "Size of the file in bytes."
        }
      }
    }
  }
}
```
