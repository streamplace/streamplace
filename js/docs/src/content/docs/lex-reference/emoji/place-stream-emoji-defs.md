---
title: place.stream.emoji.defs
description: Reference for the place.stream.emoji.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="emojiview"></a>

### `emojiView`

**Type:** `object`

**Properties:**

| Name    | Type                     | Req'd | Description                                                     | Constraints                                                                                          |
| ------- | ------------------------ | ----- | --------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `name`  | `string`                 | ✅    | Short name used to reference this emoji in chat, e.g. 'pepega'. |                                                                                                      |
| `image` | `blob`                   | ✅    | The emoji image.                                                | Accept: `image/png`, `image/gif`, `image/webp`, `image/avif`, `image/jxl`<br/>Max Size: 512000 bytes |
| `alt`   | `string`                 | ❌    | Alt text for the emoji image.                                   |                                                                                                      |
| `pack`  | [`#packView`](#packview) | ❌    | The pack this emoji belongs to.                                 |                                                                                                      |

---

<a name="packview"></a>

### `packView`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                             | Req'd | Description | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |             | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |             | Format: `cid`      |
| `author`    | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |             |                    |
| `record`    | [`place.stream.emoji.defs#emojiView`](/lex-reference/place-stream-emoji-defs#emojiview)                                                          | ✅    |             |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |             | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emoji.defs",
  "defs": {
    "emojiView": {
      "type": "object",
      "required": ["name", "image"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Short name used to reference this emoji in chat, e.g. 'pepega'."
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
          "description": "The emoji image."
        },
        "alt": {
          "type": "string",
          "description": "Alt text for the emoji image."
        },
        "pack": {
          "type": "ref",
          "ref": "#packView",
          "description": "The pack this emoji belongs to."
        }
      }
    },
    "packView": {
      "type": "object",
      "required": ["uri", "cid", "author", "record", "indexedAt"],
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
        "record": {
          "type": "ref",
          "ref": "place.stream.emoji.defs#emojiView"
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        }
      }
    }
  }
}
```
