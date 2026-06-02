---
title: place.stream.vod.comment
description: Reference for the place.stream.vod.comment lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record containing a comment on a VOD.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                                 | Req'd | Description                                                      | Constraints                             |
| ----------- | ------------------------------------------------------------------------------------ | ----- | ---------------------------------------------------------------- | --------------------------------------- |
| `text`      | `string`                                                                             | ✅    | The comment text content.                                        | Max Length: 3000<br/>Max Graphemes: 300 |
| `createdAt` | `string`                                                                             | ✅    | Client-declared timestamp when this comment was created.         | Format: `datetime`                      |
| `video`     | `string`                                                                             | ✅    | AT-URI of the place.stream.video record this comment belongs to. | Format: `at-uri`                        |
| `facets`    | Array of [`place.stream.richtext.facet`](/lex-reference/place-stream-richtext-facet) | ❌    | Annotations of text (mentions, URLs, etc)                        |                                         |
| `reply`     | [`#replyRef`](#replyref)                                                             | ❌    |                                                                  |                                         |

---

<a name="replyref"></a>

### `replyRef`

**Type:** `object`

**Properties:**

| Name     | Type                                                                                                                                   | Req'd | Description | Constraints |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `root`   | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    |             |             |
| `parent` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.comment",
  "defs": {
    "main": {
      "type": "record",
      "description": "Record containing a comment on a VOD.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["text", "createdAt", "video"],
        "properties": {
          "text": {
            "type": "string",
            "maxLength": 3000,
            "maxGraphemes": 300,
            "description": "The comment text content."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this comment was created."
          },
          "video": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.video record this comment belongs to."
          },
          "facets": {
            "type": "array",
            "description": "Annotations of text (mentions, URLs, etc)",
            "items": {
              "type": "ref",
              "ref": "place.stream.richtext.facet"
            }
          },
          "reply": {
            "type": "ref",
            "ref": "#replyRef"
          }
        }
      }
    },
    "replyRef": {
      "type": "object",
      "required": ["root", "parent"],
      "properties": {
        "root": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef"
        },
        "parent": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef"
        }
      }
    }
  }
}
```
