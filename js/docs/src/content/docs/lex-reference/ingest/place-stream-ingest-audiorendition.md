---
title: place.stream.ingest.audioRendition
description: Reference for the place.stream.ingest.audioRendition lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

An audio codec the streamer's encoder sends each of its audio tracks in (the full mix and any stems). A node keeps these locally (place.stream.local.putRecord).

**Record Key:** `any`

**Record Properties:**

| Name        | Type      | Req'd | Description              | Constraints                 |
| ----------- | --------- | ----- | ------------------------ | --------------------------- |
| `codec`     | `string`  | ✅    |                          | Known Values: `aac`, `opus` |
| `bitrate`   | `integer` | ✅    | Target bitrate, in kbps. | Min: 1                      |
| `createdAt` | `string`  | ✅    |                          | Format: `datetime`          |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.ingest.audioRendition",
  "defs": {
    "main": {
      "type": "record",
      "key": "any",
      "description": "An audio codec the streamer's encoder sends each of its audio tracks in (the full mix and any stems). A node keeps these locally (place.stream.local.putRecord).",
      "record": {
        "type": "object",
        "required": ["codec", "bitrate", "createdAt"],
        "properties": {
          "codec": {
            "type": "string",
            "knownValues": ["aac", "opus"]
          },
          "bitrate": {
            "type": "integer",
            "minimum": 1,
            "description": "Target bitrate, in kbps."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime"
          }
        }
      }
    }
  }
}
```
