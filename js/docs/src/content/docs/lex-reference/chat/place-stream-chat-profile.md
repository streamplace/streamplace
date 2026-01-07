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

| Name    | Type               | Req'd | Description | Constraints |
| ------- | ------------------ | ----- | ----------- | ----------- |
| `color` | [`#color`](#color) | ❌    |             |             |

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
          }
        }
      }
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
