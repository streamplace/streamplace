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

| Name          | Type                                                                                                                                             | Req'd | Description                                                                                                                                                                   | Constraints        |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `uri`         | `string`                                                                                                                                         | ✅    |                                                                                                                                                                               | Format: `at-uri`   |
| `cid`         | `string`                                                                                                                                         | ✅    |                                                                                                                                                                               | Format: `cid`      |
| `author`      | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                                                                                                                                                               |                    |
| `record`      | `unknown`                                                                                                                                        | ✅    |                                                                                                                                                                               |                    |
| `indexedAt`   | `string`                                                                                                                                         | ✅    |                                                                                                                                                                               | Format: `datetime` |
| `chatProfile` | [`place.stream.chat.profile`](/lex-reference/place-stream-chat-profile)                                                                          | ❌    |                                                                                                                                                                               |                    |
| `replyTo`     | Union of:<br/>&nbsp;&nbsp;[`#messageView`](#messageview)                                                                                         | ❌    |                                                                                                                                                                               |                    |
| `deleted`     | `boolean`                                                                                                                                        | ❌    | If true, this message has been deleted or labeled and should be cleared from the cache                                                                                        |                    |
| `badges`      | Array of [`place.stream.badge.defs#badgeView`](/lex-reference/place-stream-badge-defs#badgeview)                                                 | ❌    | Up to 3 badge tokens to display with the message. First badge is server-controlled, remaining badges are user-settable. Tokens are looked up in badges.json for display info. | Max Items: 3       |

---

<a name="pinnedrecordview"></a>

### `pinnedRecordView`

**Type:** `object`

View of a pinned chat record with hydrated message data.

**Properties:**

| Name        | Type                                                                              | Req'd | Description | Constraints        |
| ----------- | --------------------------------------------------------------------------------- | ----- | ----------- | ------------------ |
| `uri`       | `string`                                                                          | ✅    |             | Format: `at-uri`   |
| `cid`       | `string`                                                                          | ✅    |             | Format: `cid`      |
| `record`    | [`place.stream.chat.pinnedRecord`](/lex-reference/place-stream-chat-pinnedrecord) | ✅    |             |                    |
| `indexedAt` | `string`                                                                          | ✅    |             | Format: `datetime` |
| `pinnedBy`  | [`place.stream.chat.profile`](/lex-reference/place-stream-chat-profile)           | ❌    |             |                    |
| `message`   | [`#messageView`](#messageview)                                                    | ❌    |             |                    |

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
        },
        "badges": {
          "type": "array",
          "description": "Up to 3 badge tokens to display with the message. First badge is server-controlled, remaining badges are user-settable. Tokens are looked up in badges.json for display info.",
          "maxLength": 3,
          "items": {
            "type": "ref",
            "ref": "place.stream.badge.defs#badgeView"
          }
        }
      }
    },
    "pinnedRecordView": {
      "type": "object",
      "description": "View of a pinned chat record with hydrated message data.",
      "required": ["uri", "cid", "record", "indexedAt"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "record": {
          "type": "ref",
          "ref": "place.stream.chat.pinnedRecord"
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        },
        "pinnedBy": {
          "type": "ref",
          "ref": "place.stream.chat.profile"
        },
        "message": {
          "type": "ref",
          "ref": "#messageView"
        }
      }
    }
  }
}
```
