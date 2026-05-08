---
title: place.stream.bio.layouts.columns
description: Reference for the place.stream.bio.layouts.columns lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A column-based layout for the customizable body of a bio. On narrow viewports columns collapse into a single stack in column order.

**Properties:**

| Name      | Type                          | Req'd | Description | Constraints                   |
| --------- | ----------------------------- | ----- | ----------- | ----------------------------- |
| `columns` | Array of [`#column`](#column) | ✅    |             | Min Items: 1<br/>Max Items: 4 |

---

<a name="column"></a>

### `column`

**Type:** `object`

**Properties:**

| Name     | Type                                  | Req'd | Description                                                                                     | Constraints       |
| -------- | ------------------------------------- | ----- | ----------------------------------------------------------------------------------------------- | ----------------- |
| `width`  | `integer`                             | ❌    | Optional weight for this column's width. If omitted on any column, columns share width equally. | Min: 1<br/>Max: 4 |
| `blocks` | Array of [`#blockEntry`](#blockentry) | ✅    |                                                                                                 |                   |

---

<a name="blockentry"></a>

### `blockEntry`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Req'd | Description                                                                                                           | Constraints                     |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------- | ------------------------------- |
| `alignment` | `string`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | ❌    |                                                                                                                       | Enum: `left`, `center`, `right` |
| `colSpan`   | `integer`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | ❌    | Number of columns this block spans, starting from its column. Useful for hero banners and wide embeds. Defaults to 1. | Min: 1<br/>Max: 4               |
| `block`     | Union of:<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.text`](/lex-reference/place-stream-bio-blocks-text)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.header`](/lex-reference/place-stream-bio-blocks-header)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.image`](/lex-reference/place-stream-bio-blocks-image)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.orderedList`](/lex-reference/place-stream-bio-blocks-orderedlist)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.unorderedList`](/lex-reference/place-stream-bio-blocks-unorderedlist)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.blockquote`](/lex-reference/place-stream-bio-blocks-blockquote)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.divider`](/lex-reference/place-stream-bio-blocks-divider)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.link`](/lex-reference/place-stream-bio-blocks-link)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.socialLinks`](/lex-reference/place-stream-bio-blocks-sociallinks)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.livestream`](/lex-reference/place-stream-bio-blocks-livestream)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.schedule`](/lex-reference/place-stream-bio-blocks-schedule)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.bskyPost`](/lex-reference/place-stream-bio-blocks-bskypost)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.embed`](/lex-reference/place-stream-bio-blocks-embed) | ✅    |                                                                                                                       |                                 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.layouts.columns",
  "defs": {
    "main": {
      "type": "object",
      "description": "A column-based layout for the customizable body of a bio. On narrow viewports columns collapse into a single stack in column order.",
      "required": ["columns"],
      "properties": {
        "columns": {
          "type": "array",
          "minLength": 1,
          "maxLength": 4,
          "items": {
            "type": "ref",
            "ref": "#column"
          }
        }
      }
    },
    "column": {
      "type": "object",
      "required": ["blocks"],
      "properties": {
        "width": {
          "type": "integer",
          "minimum": 1,
          "maximum": 4,
          "description": "Optional weight for this column's width. If omitted on any column, columns share width equally."
        },
        "blocks": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#blockEntry"
          }
        }
      }
    },
    "blockEntry": {
      "type": "object",
      "required": ["block"],
      "properties": {
        "alignment": {
          "type": "string",
          "enum": ["left", "center", "right"]
        },
        "colSpan": {
          "type": "integer",
          "minimum": 1,
          "maximum": 4,
          "description": "Number of columns this block spans, starting from its column. Useful for hero banners and wide embeds. Defaults to 1."
        },
        "block": {
          "type": "union",
          "refs": [
            "place.stream.bio.blocks.text",
            "place.stream.bio.blocks.header",
            "place.stream.bio.blocks.image",
            "place.stream.bio.blocks.orderedList",
            "place.stream.bio.blocks.unorderedList",
            "place.stream.bio.blocks.blockquote",
            "place.stream.bio.blocks.divider",
            "place.stream.bio.blocks.link",
            "place.stream.bio.blocks.socialLinks",
            "place.stream.bio.blocks.livestream",
            "place.stream.bio.blocks.schedule",
            "place.stream.bio.blocks.bskyPost",
            "place.stream.bio.blocks.embed"
          ]
        }
      }
    }
  }
}
```
