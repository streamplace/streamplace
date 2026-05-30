---
title: place.stream.vod.defs
description: Reference for the place.stream.vod.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="commentview"></a>

### `commentView`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                             | Req'd | Description                      | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | -------------------------------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |                                  | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |                                  | Format: `cid`      |
| `author`    | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                  |                    |
| `record`    | `unknown`                                                                                                                                        | ✅    |                                  |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |                                  | Format: `datetime` |
| `replyTo`   | Union of:<br/>&nbsp;&nbsp;[`#commentView`](#commentview)                                                                                         | ❌    |                                  |                    |
| `likeCount` | `integer`                                                                                                                                        | ✅    | Number of likes on this comment. | Min: 0             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.defs",
  "defs": {
    "commentView": {
      "type": "object",
      "required": ["uri", "cid", "author", "record", "indexedAt", "likeCount"],
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
          "type": "unknown"
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        },
        "replyTo": {
          "type": "union",
          "refs": ["#commentView"]
        },
        "likeCount": {
          "type": "integer",
          "minimum": 0,
          "description": "Number of likes on this comment."
        }
      }
    }
  }
}
```
