---
title: place.stream.chat.defs
description: Reference for the place.stream.chat.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="messageview"></a>

### `messageView`

**Type:** `object`

**Properties:**

| Name          | Type                                                                                                                                             | Req'd | Description                                                                            | Constraints        |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | -------------------------------------------------------------------------------------- | ------------------ |
| `uri`         | `string`                                                                                                                                         | ✅    |                                                                                        | Format: `at-uri`   |
| `cid`         | `string`                                                                                                                                         | ✅    |                                                                                        | Format: `cid`      |
| `author`      | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                                                                        |                    |
| `record`      | `unknown`                                                                                                                                        | ✅    |                                                                                        |                    |
| `indexedAt`   | `string`                                                                                                                                         | ✅    |                                                                                        | Format: `datetime` |
| `chatProfile` | [`place.stream.chat.profile`](/lex-reference/place-stream-chat-profile)                                                                          | ❌    |                                                                                        |                    |
| `replyTo`     | Union of:<br/>&nbsp;&nbsp;[`#messageView`](#messageview)                                                                                         | ❌    |                                                                                        |                    |
| `deleted`     | `boolean`                                                                                                                                        | ❌    | If true, this message has been deleted or labeled and should be cleared from the cache |                    |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.chat.defs",
  "defs": {
    "messageView": {
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
          "type": "unknown"
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        },
        "chatProfile": {
          "type": "ref",
          "ref": "place.stream.chat.profile"
        },
        "replyTo": {
          "type": "union",
          "refs": ["#messageView"]
        },
        "deleted": {
          "type": "boolean",
          "description": "If true, this message has been deleted or labeled and should be cleared from the cache"
        }
      }
    }
  }
}
```
