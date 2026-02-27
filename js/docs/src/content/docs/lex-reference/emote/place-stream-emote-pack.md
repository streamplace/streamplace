---
title: place.stream.emote.pack
description: Reference for the place.stream.emote.pack lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A named collection of custom emotes available for use in chat.

**Record Key:** `tid`

**Record Properties:**

| Name          | Type     | Req'd | Description                                           | Constraints                                                                             |
| ------------- | -------- | ----- | ----------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `name`        | `string` | ✅    | Display name of the emote pack.                       | Max Length: 640<br/>Max Graphemes: 64                                                   |
| `description` | `string` | ❌    | Optional description of this emote pack.              | Max Length: 3200<br/>Max Graphemes: 320                                                 |
| `avatar`      | `blob`   | ❌    | Optional avatar image for this pack.                  | Accept: `image/png`, `image/gif`, `image/webp`, `image/avif`<br/>Max Size: 512000 bytes |
| `createdAt`   | `string` | ✅    | Client-declared timestamp when this pack was created. | Format: `datetime`                                                                      |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emote.pack",
  "defs": {
    "main": {
      "type": "record",
      "description": "A named collection of custom emotes available for use in chat.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["name", "createdAt"],
        "properties": {
          "name": {
            "type": "string",
            "maxLength": 640,
            "maxGraphemes": 64,
            "description": "Display name of the emote pack."
          },
          "description": {
            "type": "string",
            "maxLength": 3200,
            "maxGraphemes": 320,
            "description": "Optional description of this emote pack."
          },
          "avatar": {
            "type": "blob",
            "accept": ["image/png", "image/gif", "image/webp", "image/avif"],
            "maxSize": 512000,
            "description": "Optional avatar image for this pack."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this pack was created."
          }
        }
      }
    }
  }
}
```
