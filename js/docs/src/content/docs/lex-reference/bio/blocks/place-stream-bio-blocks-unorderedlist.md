---
title: place.stream.bio.blocks.unorderedList
description: Reference for the place.stream.bio.blocks.unorderedList lexicon
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

| Name                  | Type                                                                                                                                                                                                                                                                                                      | Req'd | Description                                                                                                               | Constraints |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `checked`             | `boolean`                                                                                                                                                                                                                                                                                                 | ❌    | If present, this item is a checklist item. true = checked, false = unchecked. If absent, this is a normal list item.      |             |
| `content`             | Union of:<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.text`](/lex-reference/place-stream-bio-blocks-text)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.header`](/lex-reference/place-stream-bio-blocks-header)<br/>&nbsp;&nbsp;[`place.stream.bio.blocks.image`](/lex-reference/place-stream-bio-blocks-image) | ✅    |                                                                                                                           |             |
| `children`            | Array of [`#listItem`](#listitem)                                                                                                                                                                                                                                                                         | ❌    | Nested unordered list items. Mutually exclusive with orderedListChildren; if both are present, children takes precedence. |             |
| `orderedListChildren` | [`place.stream.bio.blocks.orderedList`](/lex-reference/place-stream-bio-blocks-orderedlist)                                                                                                                                                                                                               | ❌    | Nested ordered list items. Mutually exclusive with children; if both are present, children takes precedence.              |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.unorderedList",
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
        "checked": {
          "type": "boolean",
          "description": "If present, this item is a checklist item. true = checked, false = unchecked. If absent, this is a normal list item."
        },
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
          "description": "Nested unordered list items. Mutually exclusive with orderedListChildren; if both are present, children takes precedence.",
          "items": {
            "type": "ref",
            "ref": "#listItem"
          }
        },
        "orderedListChildren": {
          "type": "ref",
          "ref": "place.stream.bio.blocks.orderedList",
          "description": "Nested ordered list items. Mutually exclusive with children; if both are present, children takes precedence."
        }
      }
    }
  }
}
```
