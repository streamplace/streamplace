---
title: place.stream.bio.blocks.embed
description: Reference for the place.stream.bio.blocks.embed lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

An embedded external website or iframe-able URL, optionally with link-preview metadata.

**Properties:**

| Name           | Type     | Req'd | Description | Constraints                                   |
| -------------- | -------- | ----- | ----------- | --------------------------------------------- |
| `src`          | `string` | ✅    |             | Format: `uri`                                 |
| `title`        | `string` | ❌    |             | Max Length: 1000<br/>Max Graphemes: 100       |
| `description`  | `string` | ❌    |             | Max Length: 3000<br/>Max Graphemes: 300       |
| `previewImage` | `blob`   | ❌    |             | Accept: `image/*`<br/>Max Size: 1000000 bytes |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.embed",
  "defs": {
    "main": {
      "type": "object",
      "description": "An embedded external website or iframe-able URL, optionally with link-preview metadata.",
      "required": ["src"],
      "properties": {
        "src": {
          "type": "string",
          "format": "uri"
        },
        "title": {
          "type": "string",
          "maxLength": 1000,
          "maxGraphemes": 100
        },
        "description": {
          "type": "string",
          "maxLength": 3000,
          "maxGraphemes": 300
        },
        "previewImage": {
          "type": "blob",
          "accept": ["image/*"],
          "maxSize": 1000000
        }
      }
    }
  }
}
```
