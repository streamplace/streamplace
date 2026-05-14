---
title: place.stream.bio.blocks.header
description: Reference for the place.stream.bio.blocks.header lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A section heading. Levels 1-3; deeper nesting is intentionally unsupported on bios.

**Properties:**

| Name        | Type                                                                                       | Req'd | Description | Constraints       |
| ----------- | ------------------------------------------------------------------------------------------ | ----- | ----------- | ----------------- |
| `plaintext` | `string`                                                                                   | ✅    |             |                   |
| `facets`    | Array of [`place.stream.bio.richtextFacet`](/lex-reference/place-stream-bio-richtextfacet) | ❌    |             |                   |
| `level`     | `integer`                                                                                  | ❌    |             | Min: 1<br/>Max: 3 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.header",
  "defs": {
    "main": {
      "type": "object",
      "description": "A section heading. Levels 1-3; deeper nesting is intentionally unsupported on bios.",
      "required": ["plaintext"],
      "properties": {
        "plaintext": {
          "type": "string"
        },
        "facets": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "place.stream.bio.richtextFacet"
          }
        },
        "level": {
          "type": "integer",
          "minimum": 1,
          "maximum": 3
        }
      }
    }
  }
}
```
