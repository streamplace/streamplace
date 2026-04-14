---
title: place.stream.emote.item
description: Reference for the place.stream.emote.item lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A single emote belonging to an emote pack.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type     | Req'd | Description                                                                                    | Constraints                                                                                          |
| ----------- | -------- | ----- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `name`      | `string` | ✅    | Short name used to reference this emote in chat. Should be alphanumeric with underscores only. | Max Length: 100<br/>Max Graphemes: 50                                                                |
| `image`     | `blob`   | ✅    | The emote image. Square images recommended.                                                    | Accept: `image/png`, `image/gif`, `image/webp`, `image/avif`, `image/jxl`<br/>Max Size: 512000 bytes |
| `alt`       | `string` | ❌    | Alt text for the emote image.                                                                  | Max Length: 2000<br/>Max Graphemes: 200                                                              |
| `creator`   | `string` | ❌    | DID of the creator/artist of this emote, if different from the pack author.                    | Format: `did`                                                                                        |
| `pack`      | `string` | ✅    | AT-URI of the place.stream.emote.pack record this emote belongs to.                            | Format: `at-uri`                                                                                     |
| `createdAt` | `string` | ✅    | Client-declared timestamp when this emote was created.                                         | Format: `datetime`                                                                                   |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emote.item",
  "defs": {
    "main": {
      "type": "record",
      "description": "A single emote belonging to an emote pack.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["name", "image", "pack", "createdAt"],
        "properties": {
          "name": {
            "type": "string",
            "maxLength": 100,
            "maxGraphemes": 50,
            "description": "Short name used to reference this emote in chat. Should be alphanumeric with underscores only."
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
            "description": "The emote image. Square images recommended."
          },
          "alt": {
            "type": "string",
            "maxLength": 2000,
            "maxGraphemes": 200,
            "description": "Alt text for the emote image."
          },
          "creator": {
            "type": "string",
            "format": "did",
            "description": "DID of the creator/artist of this emote, if different from the pack author."
          },
          "pack": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.emote.pack record this emote belongs to."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this emote was created."
          }
        }
      }
    }
  }
}
```
