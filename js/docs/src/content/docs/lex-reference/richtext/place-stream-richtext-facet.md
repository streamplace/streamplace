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

| Name       | Type                                                                                                                                                                                                                                                                                                                                                          | Req'd | Description | Constraints |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `index`    | [`app.bsky.richtext.facet#byteSlice`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#byteSlice)                                                                                                                                                                                                                    | ✅    |             |             |
| `features` | Array of Union of:<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#mention`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#mention)<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#link`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#link)<br/>&nbsp;&nbsp;[`#emote`](#emote) | ✅    |             |             |

---

<a name="emote"></a>

### `emote`

**Type:** `object`

**Properties:**

| Name   | Type                                                                                                                                   | Req'd | Description                                                                                         | Constraints                           |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | --------------------------------------------------------------------------------------------------- | ------------------------------------- |
| `name` | `string`                                                                                                                               | ✅    | Short name of the emote, e.g. 'dan'. Used as fallback text and for display before the ref resolves. | Max Length: 100<br/>Max Graphemes: 50 |
| `ref`  | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | Strong reference to the place.stream.emote.item record.                                             |                                       |

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
              "app.bsky.richtext.facet#link",
              "#emote"
            ]
          }
        }
      }
    },
    "emote": {
      "type": "object",
      "required": ["name", "ref"],
      "properties": {
        "name": {
          "type": "string",
          "maxLength": 100,
          "maxGraphemes": 50,
          "description": "Short name of the emote, e.g. 'dan'. Used as fallback text and for display before the ref resolves."
        },
        "ref": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef",
          "description": "Strong reference to the place.stream.emote.item record."
        }
      }
    }
  }
}
```
