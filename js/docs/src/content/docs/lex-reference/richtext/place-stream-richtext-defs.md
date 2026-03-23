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

| Name         | Type              | Req'd | Description                    | Constraints |
| ------------ | ----------------- | ----- | ------------------------------ | ----------- |
| `reason`     | `string`          | ❌    |                                |             |
| `categories` | Array of `string` | ❌    | Categories of censored content |             |

---

<a name="discriminatory"></a>

### `discriminatory`

**Type:** `token`

Indicates that the text has been censored due to discriminatory content.

---

<a name="sexuallyexplicit"></a>

### `sexually_explicit`

**Type:** `token`

Indicates that the text has been censored due to sexually explicit content.

---

<a name="profanity"></a>

### `profanity`

**Type:** `token`

Indicates that the text has been censored due to profanity.

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
        },
        "categories": {
          "type": "array",
          "items": {
            "type": "string",
            "knownValues": [
              "place.stream.richtext.defs#discriminatory",
              "place.stream.richtext.defs#sexually_explicit",
              "place.stream.richtext.defs#profanity"
            ]
          },
          "description": "Categories of censored content"
        }
      }
    },
    "discriminatory": {
      "type": "token",
      "description": "Indicates that the text has been censored due to discriminatory content."
    },
    "sexually_explicit": {
      "type": "token",
      "description": "Indicates that the text has been censored due to sexually explicit content."
    },
    "profanity": {
      "type": "token",
      "description": "Indicates that the text has been censored due to profanity."
    }
  }
}
```
