---
title: place.stream.bio.layouts.panels
description: Reference for the place.stream.bio.layouts.panels lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

An auto-layout of panels. Each panel is one column wide; panels flow left-to-right and wrap onto new rows. On narrow viewports panels stack vertically.

**Properties:**

| Name     | Type                        | Req'd | Description | Constraints                   |
| -------- | --------------------------- | ----- | ----------- | ----------------------------- |
| `panels` | Array of [`#panel`](#panel) | ✅    |             | Min Items: 1<br/>Max Items: 8 |

---

<a name="panel"></a>

### `panel`

**Type:** `object`

**Properties:**

| Name     | Type                                  | Req'd | Description | Constraints |
| -------- | ------------------------------------- | ----- | ----------- | ----------- |
| `blocks` | Array of [`#blockEntry`](#blockentry) | ✅    |             |             |

---

<a name="blockentry"></a>

### `blockEntry`

**Type:** `object`

**Properties:**

| Name        | Type                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Req'd | Description | Constraints                     |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ------------------------------- |
| `alignment` | `string`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | ❌    |             | Enum: `left`, `center`, `right` |
| `block`     | Union of:<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.text`](/lex-reference/place-stream-bio-blocks-text)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.header`](/lex-reference/place-stream-bio-blocks-header)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.image`](/lex-reference/place-stream-bio-blocks-image)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.orderedList`](/lex-reference/place-stream-bio-blocks-orderedlist)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.unorderedList`](/lex-reference/place-stream-bio-blocks-unorderedlist)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.blockquote`](/lex-reference/place-stream-bio-blocks-blockquote)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.divider`](/lex-reference/place-stream-bio-blocks-divider)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.link`](/lex-reference/place-stream-bio-blocks-link)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.socialLinks`](/lex-reference/place-stream-bio-blocks-sociallinks)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.livestream`](/lex-reference/place-stream-bio-blocks-livestream)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.schedule`](/lex-reference/place-stream-bio-blocks-schedule)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.bskyPost`](/lex-reference/place-stream-bio-blocks-bskypost)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.embed`](/lex-reference/place-stream-bio-blocks-embed) | ✅    |             |                                 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.layouts.panels",
  "defs": {
    "main": {
      "type": "object",
      "description": "An auto-layout of panels. Each panel is one column wide; panels flow left-to-right and wrap onto new rows. On narrow viewports panels stack vertically.",
      "required": ["panels"],
      "properties": {
        "panels": {
          "type": "array",
          "minLength": 1,
          "maxLength": 8,
          "items": {
            "type": "ref",
            "ref": "#panel"
          }
        }
      }
    },
    "panel": {
      "type": "object",
      "required": ["blocks"],
      "properties": {
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
