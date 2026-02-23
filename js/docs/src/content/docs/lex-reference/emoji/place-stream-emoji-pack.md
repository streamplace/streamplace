---
title: place.stream.emoji.pack
description: Reference for the place.stream.emoji.pack lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A named collection of custom emoji available for use in chat.

**Record Key:** `tid`

**Record Properties:**

| Name          | Type                              | Req'd | Description                                           | Constraints                             |
| ------------- | --------------------------------- | ----- | ----------------------------------------------------- | --------------------------------------- |
| `name`        | `string`                          | ✅    | Display name of the emoji pack.                       | Max Length: 640<br/>Max Graphemes: 64   |
| `description` | `string`                          | ❌    | Optional description of this emoji pack.              | Max Length: 3200<br/>Max Graphemes: 320 |
| `emoji`       | Array of [`#emojiDef`](#emojidef) | ❌    | The emoji contained in this pack.                     | Max Items: 256                          |
| `createdAt`   | `string`                          | ✅    | Client-declared timestamp when this pack was created. | Format: `datetime`                      |

---

<a name="emojidef"></a>

### `emojiDef`

**Type:** `object`

**Properties:**

| Name      | Type     | Req'd | Description                                                                                    | Constraints                                                                                          |
| --------- | -------- | ----- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `name`    | `string` | ✅    | Short name used to reference this emoji in chat. Should be alphanumeric with underscores only. | Max Length: 100<br/>Max Graphemes: 50                                                                |
| `image`   | `blob`   | ✅    | The emoji image. Square images recommended.                                                    | Accept: `image/png`, `image/gif`, `image/webp`, `image/avif`, `image/jxl`<br/>Max Size: 512000 bytes |
| `alt`     | `string` | ❌    | Alt text for the emoji image.                                                                  | Max Length: 2000<br/>Max Graphemes: 200                                                              |
| `creator` | `string` | ❌    | The creator/artist of this emoji.                                                              | Format: `did`                                                                                        |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emoji.pack",
  "defs": {
    "main": {
      "type": "record",
      "description": "A named collection of custom emoji available for use in chat.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["name", "createdAt"],
        "properties": {
          "name": {
            "type": "string",
            "maxLength": 640,
            "maxGraphemes": 64,
            "description": "Display name of the emoji pack."
          },
          "description": {
            "type": "string",
            "maxLength": 3200,
            "maxGraphemes": 320,
            "description": "Optional description of this emoji pack."
          },
          "emoji": {
            "type": "array",
            "maxLength": 256,
            "items": {
              "type": "ref",
              "ref": "#emojiDef"
            },
            "description": "The emoji contained in this pack."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this pack was created."
          }
        }
      }
    },
    "emojiDef": {
      "type": "object",
      "required": ["name", "image"],
      "properties": {
        "name": {
          "type": "string",
          "maxLength": 100,
          "maxGraphemes": 50,
          "description": "Short name used to reference this emoji in chat. Should be alphanumeric with underscores only."
        },
        "image": {
          "type": "blob",
          "accept": [
            "image/png",
            "image/gif",
            "image/webp",
            "image/avif",
            "image/jxl"
          ],
          "maxSize": 512000,
          "description": "The emoji image. Square images recommended."
        },
        "alt": {
          "type": "string",
          "maxLength": 2000,
          "maxGraphemes": 200,
          "description": "Alt text for the emoji image."
        },
        "creator": {
          "type": "string",
          "format": "did",
          "description": "The creator/artist of this emoji."
        }
      }
    }
  }
}
```
