---
title: place.stream.bio.blocks.text
description: Reference for the place.stream.bio.blocks.text lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A paragraph of rich text.

**Properties:**

| Name        | Type                                                                                 | Req'd | Description | Constraints                       |
| ----------- | ------------------------------------------------------------------------------------ | ----- | ----------- | --------------------------------- |
| `plaintext` | `string`                                                                             | ✅    |             |                                   |
| `facets`    | Array of [`place.stream.richtext.facet`](/lex-reference/place-stream-richtext-facet) | ❌    |             |                                   |
| `textSize`  | `string`                                                                             | ❌    |             | Enum: `default`, `small`, `large` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.text",
  "defs": {
    "main": {
      "type": "object",
      "description": "A paragraph of rich text.",
      "required": ["plaintext"],
      "properties": {
        "plaintext": {
          "type": "string"
        },
        "facets": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "place.stream.richtext.facet"
          }
        },
        "textSize": {
          "type": "string",
          "enum": ["default", "small", "large"]
        }
      }
    }
  }
}
```
