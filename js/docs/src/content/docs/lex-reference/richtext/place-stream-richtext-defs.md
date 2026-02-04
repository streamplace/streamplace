---
title: place.stream.richtext.defs
description: Reference for the place.stream.richtext.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="facetview"></a>

### `facetView`

**Type:** `object`

Annotation of a sub-string within rich text.

**Properties:**

| Name       | Type                                                                                                                                                                                                                                                                                                                                                            | Req'd | Description | Constraints |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `index`    | [`app.bsky.richtext.facet#byteSlice`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#byteSlice)                                                                                                                                                                                                                      | ✅    |             |             |
| `features` | Array of Union of:<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#mention`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#mention)<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#link`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#link)<br/>&nbsp;&nbsp;[`#censor`](#censor) | ✅    |             |             |

---

<a name="censor"></a>

### `censor`

**Type:** `object`

Indicates that the text in the given index has been censored.

**Properties:**

| Name     | Type     | Req'd | Description | Constraints |
| -------- | -------- | ----- | ----------- | ----------- |
| `reason` | `string` | ❌    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.richtext.defs",
  "defs": {
    "facetView": {
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
              "#censor"
            ]
          }
        }
      }
    },
    "censor": {
      "type": "object",
      "description": "Indicates that the text in the given index has been censored.",
      "properties": {
        "reason": {
          "type": "string"
        }
      }
    }
  }
}
```
