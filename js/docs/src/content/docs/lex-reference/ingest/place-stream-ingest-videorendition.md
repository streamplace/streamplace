---
title: place.stream.ingest.videoRendition
description: Reference for the place.stream.ingest.videoRendition lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A video track the streamer's encoder sends to the node they stream to, which tells the encoder about it when it connects (the Streamplace OBS plugin asks). A node keeps these locally (place.stream.local.putRecord).

**Record Key:** `any`

**Record Properties:**

| Name        | Type                                                                                      | Req'd | Description              | Constraints                 |
| ----------- | ----------------------------------------------------------------------------------------- | ----- | ------------------------ | --------------------------- |
| `codec`     | `string`                                                                                  | ✅    |                          | Known Values: `h264`, `av1` |
| `width`     | `integer`                                                                                 | ✅    |                          | Min: 1                      |
| `height`    | `integer`                                                                                 | ✅    |                          | Min: 1                      |
| `framerate` | [`place.stream.ingest.defs#framerate`](/lex-reference/place-stream-ingest-defs#framerate) | ✅    |                          |                             |
| `bitrate`   | `integer`                                                                                 | ✅    | Target bitrate, in kbps. | Min: 1                      |
| `createdAt` | `string`                                                                                  | ✅    |                          | Format: `datetime`          |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.ingest.videoRendition",
  "defs": {
    "main": {
      "type": "record",
      "key": "any",
      "description": "A video track the streamer's encoder sends to the node they stream to, which tells the encoder about it when it connects (the Streamplace OBS plugin asks). A node keeps these locally (place.stream.local.putRecord).",
      "record": {
        "type": "object",
        "required": [
          "codec",
          "width",
          "height",
          "framerate",
          "bitrate",
          "createdAt"
        ],
        "properties": {
          "codec": {
            "type": "string",
            "knownValues": ["h264", "av1"]
          },
          "width": {
            "type": "integer",
            "minimum": 1
          },
          "height": {
            "type": "integer",
            "minimum": 1
          },
          "framerate": {
            "type": "ref",
            "ref": "place.stream.ingest.defs#framerate"
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
