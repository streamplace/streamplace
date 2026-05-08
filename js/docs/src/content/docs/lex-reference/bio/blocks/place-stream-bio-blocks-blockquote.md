---
title: place.stream.bio.blocks.blockquote
description: Reference for the place.stream.bio.blocks.blockquote lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A block-level quotation.

**Properties:**

| Name        | Type                                                                                 | Req'd | Description | Constraints |
| ----------- | ------------------------------------------------------------------------------------ | ----- | ----------- | ----------- |
| `plaintext` | `string`                                                                             | ✅    |             |             |
| `facets`    | Array of [`place.stream.richtext.facet`](/lex-reference/place-stream-richtext-facet) | ❌    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.blockquote",
  "defs": {
    "main": {
      "type": "object",
      "description": "A block-level quotation.",
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
        }
      }
    }
  }
}
```
