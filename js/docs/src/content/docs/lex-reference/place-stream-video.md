---
title: place.stream.video
description: Reference for the place.stream.video lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Some audiovisual content.

**Record Key:** `tid`

**Record Properties:**

| Name         | Type                                                                                                                                                                                                                  | Req'd | Description                                                                                        | Constraints                             |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `creator`    | `string`                                                                                                                                                                                                              | ✅    | The DID of the creator of this video.                                                              | Format: `did`                           |
| `createdAt`  | `string`                                                                                                                                                                                                              | ✅    | When this video started recording.                                                                 | Format: `datetime`                      |
| `title`      | `string`                                                                                                                                                                                                              | ❌    |                                                                                                    | Max Length: 1400<br/>Max Graphemes: 140 |
| `source`     | Union of:<br/>&nbsp;&nbsp;[`place.stream.muxl.defs#archive`](/lex-reference/place-stream-muxl-defs#archive)<br/>&nbsp;&nbsp;[`place.stream.muxl.defs#archiveBlob`](/lex-reference/place-stream-muxl-defs#archiveblob) | ✅    | The source media for this video.                                                                   |                                         |
| `duration`   | `integer`                                                                                                                                                                                                             | ❌    | Total duration of the video in nanoseconds.                                                        |                                         |
| `livestream` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined)                                                                                | ❌    | Back-reference to the place.stream.livestream record, if this video originated from a live stream. |                                         |

---

<a name="videoview"></a>

### `videoView`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                             | Req'd | Description | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------- | ------------------ |
| `uri`       | `string`                                                                                                                                         | ✅    |             | Format: `at-uri`   |
| `cid`       | `string`                                                                                                                                         | ✅    |             | Format: `cid`      |
| `creator`   | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |             |                    |
| `record`    | `unknown`                                                                                                                                        | ✅    |             |                    |
| `indexedAt` | `string`                                                                                                                                         | ✅    |             | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.video",
  "defs": {
    "main": {
      "type": "record",
      "description": "Some audiovisual content.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["creator", "createdAt", "source"],
        "properties": {
          "creator": {
            "type": "string",
            "format": "did",
            "description": "The DID of the creator of this video."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "When this video started recording."
          },
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140
          },
          "source": {
            "type": "union",
            "refs": [
              "place.stream.muxl.defs#archive",
              "place.stream.muxl.defs#archiveBlob"
            ],
            "description": "The source media for this video."
          },
          "duration": {
            "type": "integer",
            "description": "Total duration of the video in nanoseconds."
          },
          "livestream": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "Back-reference to the place.stream.livestream record, if this video originated from a live stream."
          }
        }
      }
    },
    "videoView": {
      "type": "object",
      "required": ["uri", "cid", "creator", "record", "indexedAt"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "creator": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic"
        },
        "record": {
          "type": "unknown"
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
