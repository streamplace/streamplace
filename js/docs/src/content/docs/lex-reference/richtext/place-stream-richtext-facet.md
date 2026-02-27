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

| Name   | Type     | Req'd | Description                                                                                    | Constraints                           |
| ------ | -------- | ----- | ---------------------------------------------------------------------------------------------- | ------------------------------------- |
| `name` | `string` | ✅    | Short name used to reference this emoji in chat. Should be alphanumeric with underscores only. | Max Length: 100<br/>Max Graphemes: 50 |
| `url`  | `string` | ✅    | URL where the image for this emoji can be retrieved.                                           | Format: `uri`                         |

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
      "required": ["name", "url"],
      "properties": {
        "name": {
          "type": "string",
          "maxLength": 100,
          "maxGraphemes": 50,
          "description": "Short name used to reference this emoji in chat. Should be alphanumeric with underscores only."
        },
        "url": {
          "type": "string",
          "format": "uri",
          "description": "URL where the image for this emoji can be retrieved."
        }
      }
    }
  }
}
```
