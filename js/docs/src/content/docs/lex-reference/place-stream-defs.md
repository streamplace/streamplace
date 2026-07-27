---
title: place.stream.defs
description: Reference for the place.stream.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="blockview"></a>

### `blockView`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                             | Req'd | Description | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |             | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |             | Format: `cid`      |
| `blocker`   | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |             |                    |
| `record`    | [`app.bsky.graph.block`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/graph/block.json#undefined)                       | ✅    |             |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |             | Format: `datetime` |

---

<a name="renditions"></a>

### `renditions`

**Type:** `object`

**Properties:**

| Name         | Type                                | Req'd | Description | Constraints |
| ------------ | ----------------------------------- | ----- | ----------- | ----------- |
| `renditions` | Array of [`#rendition`](#rendition) | ✅    |             |             |

---

<a name="rendition"></a>

### `rendition`

**Type:** `object`

**Properties:**

| Name      | Type      | Req'd | Description                               | Constraints |
| --------- | --------- | ----- | ----------------------------------------- | ----------- |
| `name`    | `string`  | ✅    |                                           |             |
| `bitrate` | `integer` | ❌    | Nominal video bitrate in bits per second. |             |
| `width`   | `integer` | ❌    |                                           |             |
| `height`  | `integer` | ❌    |                                           |             |

---

<a name="activitygame"></a>

### `activityGame`

**Type:** `object`

A game from the gamesgamesgamesgames catalog, identified by its AT URI.

**Properties:**

| Name   | Type     | Req'd | Description                      | Constraints      |
| ------ | -------- | ----- | -------------------------------- | ---------------- |
| `uri`  | `string` | ✅    |                                  | Format: `at-uri` |
| `name` | `string` | ❌    | Cached display name of the game. |                  |

---

<a name="activitylabel"></a>

### `activityLabel`

**Type:** `object`

A non-game activity with a well-known label.

**Properties:**

| Name    | Type     | Req'd | Description | Constraints                                                                                                                                            |
| ------- | -------- | ----- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `label` | `string` | ✅    |             | Known Values: `events`, `just_chatting`, `podcasting`, `music`, `art`, `software_dev`, `cooking`, `miniatures`, `makers_crafting`, `fitness`, `sports` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.defs",
  "defs": {
    "blockView": {
      "type": "object",
      "required": ["uri", "cid", "blocker", "record", "indexedAt"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "blocker": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic"
        },
        "record": {
          "type": "ref",
          "ref": "app.bsky.graph.block"
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        }
      }
    },
    "renditions": {
      "type": "object",
      "required": ["renditions"],
      "properties": {
        "renditions": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#rendition"
          }
        }
      }
    },
    "rendition": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": {
          "type": "string"
        },
        "bitrate": {
          "type": "integer",
          "description": "Nominal video bitrate in bits per second."
        },
        "width": {
          "type": "integer"
        },
        "height": {
          "type": "integer"
        }
      }
    },
    "activityGame": {
      "type": "object",
      "description": "A game from the gamesgamesgamesgames catalog, identified by its AT URI.",
      "required": ["uri"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "name": {
          "type": "string",
          "description": "Cached display name of the game."
        }
      }
    },
    "activityLabel": {
      "type": "object",
      "description": "A non-game activity with a well-known label.",
      "required": ["label"],
      "properties": {
        "label": {
          "type": "string",
          "knownValues": [
            "events",
            "just_chatting",
            "podcasting",
            "music",
            "art",
            "software_dev",
            "cooking",
            "miniatures",
            "makers_crafting",
            "fitness",
            "sports"
          ]
        }
      }
    }
  }
}
```
