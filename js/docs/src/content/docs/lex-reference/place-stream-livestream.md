---
title: place.stream.livestream
description: Reference for the place.stream.livestream lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record announcing a livestream is happening

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                                                                                   | Req'd | Description                                                                                                                      | Constraints                             |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `title`     | `string`                                                                                                                               | ✅    | The title of the livestream, as it will be announced to followers.                                                               | Max Length: 1400<br/>Max Graphemes: 140 |
| `url`       | `string`                                                                                                                               | ❌    | The URL where this stream can be found. This is primarily a hint for other Streamplace nodes to locate and replicate the stream. | Format: `uri`                           |
| `createdAt` | `string`                                                                                                                               | ✅    | Client-declared timestamp when this livestream started.                                                                          | Format: `datetime`                      |
| `post`      | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | The post that announced this livestream. Used for chat replies.                                                                  |                                         |

---

<a name="livestreamview"></a>

### `livestreamView`

**Type:** `object`

**Properties:**

| Name          | Type                                                                                                                                             | Req'd | Description                                                                                              | Constraints        |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | -------------------------------------------------------------------------------------------------------- | ------------------ |
| `uri`         | `string`                                                                                                                                         | ✅    |                                                                                                          | Format: `at-uri`   |
| `cid`         | `string`                                                                                                                                         | ✅    |                                                                                                          | Format: `cid`      |
| `author`      | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                                                                                          |                    |
| `record`      | `unknown`                                                                                                                                        | ✅    |                                                                                                          |                    |
| `indexedAt`   | `string`                                                                                                                                         | ✅    |                                                                                                          | Format: `datetime` |
| `viewerCount` | [`place.stream.livestream#viewerCount`](/lex-reference/place-stream-livestream#viewercount)                                                      | ❌    | The number of viewers watching this livestream. Use when you can't reasonably use #viewerCount directly. |                    |

---

<a name="viewercount"></a>

### `viewerCount`

**Type:** `object`

**Properties:**

| Name    | Type      | Req'd | Description | Constraints |
| ------- | --------- | ----- | ----------- | ----------- |
| `count` | `integer` | ✅    |             |             |

---

<a name="streamplaceanything"></a>

### `streamplaceAnything`

**Type:** `object`

**Properties:**

| Name         | Type                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Req'd | Description | Constraints |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `livestream` | Union of:<br/>&nbsp;&nbsp;[`#livestreamView`](#livestreamview)<br/>&nbsp;&nbsp;[`#viewerCount`](#viewercount)<br/>&nbsp;&nbsp;[`place.stream.defs#blockView`](/lex-reference/place-stream-defs#blockview)<br/>&nbsp;&nbsp;[`place.stream.defs#renditions`](/lex-reference/place-stream-defs#renditions)<br/>&nbsp;&nbsp;[`place.stream.defs#rendition`](/lex-reference/place-stream-defs#rendition)<br/>&nbsp;&nbsp;[`place.stream.chat.defs#messageView`](/lex-reference/place-stream-chat-defs#messageview) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.livestream",
  "defs": {
    "main": {
      "type": "record",
      "description": "Record announcing a livestream is happening",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["title", "createdAt"],
        "properties": {
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140,
            "description": "The title of the livestream, as it will be announced to followers."
          },
          "url": {
            "type": "string",
            "format": "uri",
            "description": "The URL where this stream can be found. This is primarily a hint for other Streamplace nodes to locate and replicate the stream."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this livestream started."
          },
          "post": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The post that announced this livestream. Used for chat replies."
          }
        }
      }
    },
    "livestreamView": {
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
        "viewerCount": {
          "type": "ref",
          "description": "The number of viewers watching this livestream. Use when you can't reasonably use #viewerCount directly.",
          "ref": "place.stream.livestream#viewerCount"
        }
      }
    },
    "viewerCount": {
      "type": "object",
      "required": ["count"],
      "properties": {
        "count": {
          "type": "integer"
        }
      }
    },
    "streamplaceAnything": {
      "type": "object",
      "required": ["livestream"],
      "properties": {
        "livestream": {
          "type": "union",
          "refs": [
            "#livestreamView",
            "#viewerCount",
            "place.stream.defs#blockView",
            "place.stream.defs#renditions",
            "place.stream.defs#rendition",
            "place.stream.chat.defs#messageView"
          ]
        }
      }
    }
  }
}
```
