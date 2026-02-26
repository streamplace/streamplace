---
title: place.stream.richtext.facet
description: Reference for the place.stream.richtext.facet lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

Annotation of a sub-string within rich text.

**Properties:**

| Name       | Type                                                                                                                                                                                                                                                                                                                       | Req'd | Description | Constraints |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `index`    | [`app.bsky.richtext.facet#byteSlice`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#byteSlice)                                                                                                                                                                                 | ✅    |             |             |
| `features` | Array of Union of:<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#mention`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#mention)<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#link`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#link) | ✅    |             |             |

---

<a name="emoji"></a>

### `emoji`

**Type:** `object`

**Properties:**

| Name   | Type     | Req'd | Description                                                                                    | Constraints |
| ------ | -------- | ----- | ---------------------------------------------------------------------------------------------- | ----------- |
| `name` | `string` | ❌    | Short name used to reference this emoji in chat. Should be alphanumeric with underscores only. |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.richtext.facet",
  "defs": {
    "main": {
      "type": "object",
      "description": "Annotation of a sub-string within rich text.",
      "required": ["index", "features"],
      "properties": {
        "index": {
          "type": "ref",
          "ref": "app.bsky.richtext.facet#byteSlice"
        },
        "features": {
          "type": "array",
          "items": {
            "type": "union",
            "refs": [
              "app.bsky.richtext.facet#mention",
              "app.bsky.richtext.facet#link"
            ]
          }
        }
      }
    },
    "emoji": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "description": "Short name used to reference this emoji in chat. Should be alphanumeric with underscores only."
        }
      }
    }
  }
}
```
