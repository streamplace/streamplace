---
title: place.stream.bio.blocks.link
description: Reference for the place.stream.bio.blocks.link lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A labeled outbound link, optionally with a preview image. Used for buttons, donation cards, channel callouts, etc.

**Properties:**

| Name           | Type     | Req'd | Description                             | Constraints                                   |
| -------------- | -------- | ----- | --------------------------------------- | --------------------------------------------- |
| `url`          | `string` | ✅    |                                         | Format: `uri`                                 |
| `text`         | `string` | ❌    | Display label for the link.             | Max Length: 1000<br/>Max Graphemes: 100       |
| `description`  | `string` | ❌    | Secondary text shown beneath the label. | Max Length: 3000<br/>Max Graphemes: 300       |
| `previewImage` | `blob`   | ❌    |                                         | Accept: `image/*`<br/>Max Size: 1000000 bytes |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.link",
  "defs": {
    "main": {
      "type": "object",
      "description": "A labeled outbound link, optionally with a preview image. Used for buttons, donation cards, channel callouts, etc.",
      "required": ["url"],
      "properties": {
        "url": {
          "type": "string",
          "format": "uri"
        },
        "text": {
          "type": "string",
          "description": "Display label for the link.",
          "maxLength": 1000,
          "maxGraphemes": 100
        },
        "description": {
          "type": "string",
          "description": "Secondary text shown beneath the label.",
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
