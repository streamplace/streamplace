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

| Name    | Type                     | Req'd | Description                                                     | Constraints                                                                                          |
| ------- | ------------------------ | ----- | --------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `name`  | `string`                 | ✅    | Short name used to reference this emote in chat, e.g. 'pepega'. |                                                                                                      |
| `image` | `blob`                   | ✅    | The emote image.                                                | Accept: `image/png`, `image/gif`, `image/webp`, `image/avif`, `image/jxl`<br/>Max Size: 512000 bytes |
| `alt`   | `string`                 | ❌    | Alt text for the emote image.                                   |                                                                                                      |
| `pack`  | [`#packView`](#packview) | ❌    | The pack this emote belongs to.                                 |                                                                                                      |

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
| `record`    | [`place.stream.emote.defs#emoteView`](/lex-reference/place-stream-emote-defs#emoteview)                                                          | ✅    |             |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |             | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emote.defs",
  "defs": {
    "emoteView": {
      "type": "object",
      "required": ["name", "image"],
      "properties": {
        "name": {
          "type": "string",
          "description": "Short name used to reference this emote in chat, e.g. 'pepega'."
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
          "description": "The emote image."
        },
        "alt": {
          "type": "string",
          "description": "Alt text for the emote image."
        },
        "pack": {
          "type": "ref",
          "ref": "#packView",
          "description": "The pack this emote belongs to."
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
          "ref": "place.stream.emote.defs#emoteView"
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
