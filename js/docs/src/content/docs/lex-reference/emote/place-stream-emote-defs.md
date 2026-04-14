---
title: place.stream.emote.defs
description: Reference for the place.stream.emote.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="emoteview"></a>

### `emoteView`

**Type:** `object`

**Properties:**

| Name        | Type     | Req'd | Description                                                                 | Constraints        |
| ----------- | -------- | ----- | --------------------------------------------------------------------------- | ------------------ |
| `uri`       | `string` | ✅    | AT-URI of the place.stream.emote.item record.                               | Format: `at-uri`   |
| `cid`       | `string` | ✅    |                                                                             | Format: `cid`      |
| `name`      | `string` | ✅    | Short name used to reference this emote in chat.                            |                    |
| `imageUrl`  | `string` | ✅    | Resolved URL for the emote image.                                           | Format: `uri`      |
| `alt`       | `string` | ❌    | Alt text for the emote image.                                               |                    |
| `creator`   | `string` | ❌    | DID of the creator/artist of this emote, if different from the pack author. | Format: `did`      |
| `indexedAt` | `string` | ✅    |                                                                             | Format: `datetime` |

---

<a name="packview"></a>

### `packView`

**Type:** `object`

**Properties:**

| Name           | Type                                                                                                                                             | Req'd | Description                                        | Constraints            |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | -------------------------------------------------- | ---------------------- |
| `uri`          | `string`                                                                                                                                         | ✅    |                                                    | Format: `at-uri`       |
| `cid`          | `string`                                                                                                                                         | ✅    |                                                    | Format: `cid`          |
| `author`       | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                                    |                        |
| `name`         | `string`                                                                                                                                         | ✅    |                                                    |                        |
| `description`  | `string`                                                                                                                                         | ❌    |                                                    |                        |
| `emotes`       | Array of [`#emoteView`](#emoteview)                                                                                                              | ✅    |                                                    |                        |
| `indexedAt`    | `string`                                                                                                                                         | ✅    |                                                    | Format: `datetime`     |
| `relationship` | `string`                                                                                                                                         | ❌    | Why this pack is available to the requesting user. | Known Values: `follow` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emote.defs",
  "defs": {
    "emoteView": {
      "type": "object",
      "required": ["uri", "cid", "name", "imageUrl", "indexedAt"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri",
          "description": "AT-URI of the place.stream.emote.item record."
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "name": {
          "type": "string",
          "description": "Short name used to reference this emote in chat."
        },
        "imageUrl": {
          "type": "string",
          "format": "uri",
          "description": "Resolved URL for the emote image."
        },
        "alt": {
          "type": "string",
          "description": "Alt text for the emote image."
        },
        "creator": {
          "type": "string",
          "format": "did",
          "description": "DID of the creator/artist of this emote, if different from the pack author."
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        }
      }
    },
    "packView": {
      "type": "object",
      "required": ["uri", "cid", "author", "name", "emotes", "indexedAt"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "author": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic"
        },
        "name": {
          "type": "string"
        },
        "description": {
          "type": "string"
        },
        "emotes": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#emoteView"
          }
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        },
        "relationship": {
          "type": "string",
          "description": "Why this pack is available to the requesting user.",
          "knownValues": ["follow"]
        }
      }
    }
  }
}
```
