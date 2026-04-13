---
title: place.stream.chat.profile
description: Reference for the place.stream.chat.profile lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record containing customizations for a user's chat profile.

**Record Key:** `literal:self`

**Record Properties:**

| Name         | Type                                                                                                                                            | Req'd | Description                                                                                                                          | Constraints   |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------ | ------------- |
| `color`      | [`#color`](#color)                                                                                                                              | ❌    |                                                                                                                                      |               |
| `selfLabels` | Array of [`#selfLabel`](#selflabel)                                                                                                             | ❌    | Self-applied labels for this profile, e.g. 'bot'.                                                                                    | Max Items: 10 |
| `selection`  | Array of [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | Badge issuances this user has selected to display in chat. Each entry is a strong reference to a place.stream.badge.issuance record. | Max Items: 10 |

---

<a name="selflabel"></a>

### `selfLabel`

**Type:** `string`

Label that a user can apply to their own profile.

**Constraints:**<br/>Known Values: `bot`

---

<a name="color"></a>

### `color`

**Type:** `object`

Customizations for the color of a user's name in chat

**Properties:**

| Name    | Type      | Req'd | Description | Constraints         |
| ------- | --------- | ----- | ----------- | ------------------- |
| `red`   | `integer` | ✅    |             | Min: 0<br/>Max: 255 |
| `green` | `integer` | ✅    |             | Min: 0<br/>Max: 255 |
| `blue`  | `integer` | ✅    |             | Min: 0<br/>Max: 255 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.chat.profile",
  "defs": {
    "main": {
      "type": "record",
      "description": "Record containing customizations for a user's chat profile.",
      "key": "literal:self",
      "record": {
        "type": "object",
        "required": [],
        "properties": {
          "color": {
            "type": "ref",
            "ref": "#color"
          },
          "selfLabels": {
            "type": "array",
            "description": "Self-applied labels for this profile, e.g. 'bot'.",
            "maxLength": 10,
            "items": {
              "type": "ref",
              "ref": "#selfLabel"
            }
          },
          "selection": {
            "type": "array",
            "description": "Badge issuances this user has selected to display in chat. Each entry is a strong reference to a place.stream.badge.issuance record.",
            "maxLength": 10,
            "items": {
              "type": "ref",
              "ref": "com.atproto.repo.strongRef"
            }
          }
        }
      }
    },
    "selfLabel": {
      "type": "string",
      "description": "Label that a user can apply to their own profile.",
      "knownValues": ["bot"]
    },
    "color": {
      "type": "object",
      "description": "Customizations for the color of a user's name in chat",
      "required": ["red", "green", "blue"],
      "properties": {
        "red": {
          "type": "integer",
          "minimum": 0,
          "maximum": 255
        },
        "green": {
          "type": "integer",
          "minimum": 0,
          "maximum": 255
        },
        "blue": {
          "type": "integer",
          "minimum": 0,
          "maximum": 255
        }
      }
    }
  }
}
```
