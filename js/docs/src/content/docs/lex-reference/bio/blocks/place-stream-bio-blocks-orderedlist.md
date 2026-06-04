---
title: place.stream.bio.blocks.orderedList
description: Reference for the place.stream.bio.blocks.orderedList lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

**Properties:**

| Name       | Type                              | Req'd | Description | Constraints |
| ---------- | --------------------------------- | ----- | ----------- | ----------- |
| `children` | Array of [`#listItem`](#listitem) | ✅    |             |             |

---

<a name="listitem"></a>

### `listItem`

**Type:** `object`

**Properties:**

| Name                    | Type                                                                                                                                                                                                                                                                                                      | Req'd | Description                                                                                                               | Constraints |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `content`               | Union of:<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.text`](/lex-reference/place-stream-bio-blocks-text)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.header`](/lex-reference/place-stream-bio-blocks-header)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.image`](/lex-reference/place-stream-bio-blocks-image) | ✅    |                                                                                                                           |             |
| `children`              | Array of [`#listItem`](#listitem)                                                                                                                                                                                                                                                                         | ❌    | Nested ordered list items. Mutually exclusive with unorderedListChildren; if both are present, children takes precedence. |             |
| `unorderedListChildren` | [`place.stream.bio.blocks.unorderedList`](/lex-reference/place-stream-bio-blocks-unorderedlist)                                                                                                                                                                                                           | ❌    | Nested unordered list items. Mutually exclusive with children; if both are present, children takes precedence.            |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.orderedList",
  "defs": {
    "main": {
      "type": "object",
      "required": ["children"],
      "properties": {
        "children": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#listItem"
          }
        }
      }
    },
    "listItem": {
      "type": "object",
      "required": ["content"],
      "properties": {
        "content": {
          "type": "union",
          "refs": [
            "place.stream.bio.blocks.text",
            "place.stream.bio.blocks.header",
            "place.stream.bio.blocks.image"
          ]
        },
        "children": {
          "type": "array",
          "description": "Nested ordered list items. Mutually exclusive with unorderedListChildren; if both are present, children takes precedence.",
          "items": {
            "type": "ref",
            "ref": "#listItem"
          }
        },
        "unorderedListChildren": {
          "type": "ref",
          "ref": "place.stream.bio.blocks.unorderedList",
          "description": "Nested unordered list items. Mutually exclusive with children; if both are present, children takes precedence."
        }
      }
    }
  }
}
```
