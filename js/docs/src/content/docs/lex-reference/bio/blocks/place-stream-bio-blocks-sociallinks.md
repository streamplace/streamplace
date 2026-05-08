---
title: place.stream.bio.blocks.socialLinks
description: Reference for the place.stream.bio.blocks.socialLinks lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A group of social/platform links rendered as an icon row.

**Properties:**

| Name    | Type                                                                                   | Req'd | Description | Constraints                    |
| ------- | -------------------------------------------------------------------------------------- | ----- | ----------- | ------------------------------ |
| `links` | Array of [`place.stream.bio.defs#social`](/lex-reference/place-stream-bio-defs#social) | ✅    |             | Min Items: 1<br/>Max Items: 32 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.socialLinks",
  "defs": {
    "main": {
      "type": "object",
      "description": "A group of social/platform links rendered as an icon row.",
      "required": ["links"],
      "properties": {
        "links": {
          "type": "array",
          "minLength": 1,
          "maxLength": 32,
          "items": {
            "type": "ref",
            "ref": "place.stream.bio.defs#social"
          }
        }
      }
    }
  }
}
```
