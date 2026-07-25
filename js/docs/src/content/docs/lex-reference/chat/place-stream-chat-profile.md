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

| Name         | Type                                   | Req'd | Description                                       | Constraints   |
| ------------ | -------------------------------------- | ----- | ------------------------------------------------- | ------------- |
| `color`      | [`#color`](#color)                     | ❌    |                                                   |               |
| `selfLabels` | Array of [`#selfLabel`](#selflabel)    | ❌    | Self-applied labels for this profile, e.g. 'bot'. | Max Items: 10 |
| `badges`     | [`#badgeSelections`](#badgeselections) | ❌    | Badge selections for display in chat.             |               |

---

<a name="badgeselections"></a>

### `badgeSelections`

**Type:** `object`

Selected badges for display in chat, organized by slot.

**Properties:**

| Name       | Type                                                                                                                                   | Req'd | Description                                                | Constraints   |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ---------------------------------------------------------- | ------------- |
| `streamer` | Array of [`#streamerBadgeSelection`](#streamerbadgeselection)                                                                          | ❌    | Selected streamer-issued badges, one per streamer channel. | Max Items: 20 |
| `global`   | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | Selected globally-issued badge (e.g. event badge).         |               |

---

<a name="streamerbadgeselection"></a>

### `streamerBadgeSelection`

**Type:** `object`

A selected badge for a specific streamer's channel.

**Properties:**

| Name       | Type                                                                                                                                   | Req'd | Description                                                          | Constraints   |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------- | ------------- |
| `streamer` | `string`                                                                                                                               | ✅    | DID of the streamer whose channel this selection applies to.         | Format: `did` |
| `badge`    | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | Strong reference to the selected place.stream.badge.issuance record. |               |

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
          "badges": {
            "type": "ref",
            "ref": "#badgeSelections",
            "description": "Badge selections for display in chat."
          }
        }
      }
    },
    "badgeSelections": {
      "type": "object",
      "description": "Selected badges for display in chat, organized by slot.",
      "properties": {
        "streamer": {
          "type": "array",
          "description": "Selected streamer-issued badges, one per streamer channel.",
          "maxLength": 20,
          "items": {
            "type": "ref",
            "ref": "#streamerBadgeSelection"
          }
        },
        "global": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef",
          "description": "Selected globally-issued badge (e.g. event badge)."
        }
      }
    },
    "streamerBadgeSelection": {
      "type": "object",
      "description": "A selected badge for a specific streamer's channel.",
      "required": ["streamer", "badge"],
      "properties": {
        "streamer": {
          "type": "string",
          "format": "did",
          "description": "DID of the streamer whose channel this selection applies to."
        },
        "badge": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef",
          "description": "Strong reference to the selected place.stream.badge.issuance record."
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
