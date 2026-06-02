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

| Name        | Type                                                                                                                                             | Req'd | Description                                                                                                                                                                                           | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |                                                                                                                                                                                                       | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |                                                                                                                                                                                                       | Format: `cid`      |
| `author`    | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                                                                                                                                                                                       |                    |
| `record`    | `unknown`                                                                                                                                        | ✅    |                                                                                                                                                                                                       |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |                                                                                                                                                                                                       | Format: `datetime` |
| `replyTo`   | Union of:<br/>&nbsp;&nbsp;[`#commentViewBasic`](#commentviewbasic)                                                                               | ❌    | The parent comment this one replies to, if any. A non-recursive view (it carries no replyTo of its own), so the thread is flattened to a single hop; walk `record.reply` to follow the chain further. |                    |
| `likeCount` | `integer`                                                                                                                                        | ✅    | Number of likes on this comment.                                                                                                                                                                      | Min: 0             |

---

<a name="commentviewbasic"></a>

### `commentViewBasic`

**Type:** `object`

A comment view without its own `replyTo`, used to represent the parent of a reply without recursively nesting the whole thread.

**Properties:**

| Name        | Type                                                                                                                                             | Req'd | Description                      | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | -------------------------------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |                                  | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |                                  | Format: `cid`      |
| `author`    | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                  |                    |
| `record`    | `unknown`                                                                                                                                        | ✅    |                                  |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |                                  | Format: `datetime` |
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
          "refs": ["#commentViewBasic"],
          "description": "The parent comment this one replies to, if any. A non-recursive view (it carries no replyTo of its own), so the thread is flattened to a single hop; walk `record.reply` to follow the chain further."
        },
        "likeCount": {
          "type": "integer",
          "minimum": 0,
          "description": "Number of likes on this comment."
        }
      }
    },
    "commentViewBasic": {
      "type": "object",
      "description": "A comment view without its own `replyTo`, used to represent the parent of a reply without recursively nesting the whole thread.",
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
