---
title: place.stream.caption.importCaptions
description: Reference for the place.stream.caption.importCaptions lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Import a WebVTT or SRT caption file for one of the caller's videos, writing place.stream.caption.transcript records to the caller's repo. Replaces the caller's earlier imported or human-authored records for the same video and language.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description | Constraints                           |
| ---------- | -------- | ----- | ----------- | ------------------------------------- |
| `video`    | `string` | ✅    |             | Format: `at-uri`                      |
| `language` | `string` | ✅    |             | Format: `language`                    |
| `kind`     | `string` | ❌    |             | Known Values: `captions`, `subtitles` |
| `format`   | `string` | ✅    |             | Known Values: `vtt`, `srt`            |
| `body`     | `string` | ✅    |             | Max Length: 5000000                   |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name   | Type              | Req'd | Description | Constraints |
| ------ | ----------------- | ----- | ----------- | ----------- |
| `uris` | Array of `string` | ✅    |             |             |

**Possible Errors:**

- `NotFound`
- `Forbidden`
- `InvalidCaptions`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.caption.importCaptions",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Import a WebVTT or SRT caption file for one of the caller's videos, writing place.stream.caption.transcript records to the caller's repo. Replaces the caller's earlier imported or human-authored records for the same video and language.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["video", "language", "format", "body"],
          "properties": {
            "video": {
              "type": "string",
              "format": "at-uri"
            },
            "language": {
              "type": "string",
              "format": "language"
            },
            "kind": {
              "type": "string",
              "knownValues": ["captions", "subtitles"]
            },
            "format": {
              "type": "string",
              "knownValues": ["vtt", "srt"]
            },
            "body": {
              "type": "string",
              "maxLength": 5000000
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uris"],
          "properties": {
            "uris": {
              "type": "array",
              "items": {
                "type": "string",
                "format": "at-uri"
              }
            }
          }
        }
      },
      "errors": [
        {
          "name": "NotFound"
        },
        {
          "name": "Forbidden"
        },
        {
          "name": "InvalidCaptions"
        }
      ]
    }
  }
}
```
