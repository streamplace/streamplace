---
title: place.stream.metadata.configuration
description: Reference for the place.stream.metadata.configuration lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Default metadata record for livestream including content warnings, rights, and
distribution policy

**Record Key:** `literal:self`

**Record Properties:**

| Name                 | Type                                                                                                  | Req'd | Description | Constraints |
| -------------------- | ----------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `contentWarnings`    | [`place.stream.metadata.contentWarnings`](/lex-reference/place-stream-metadata-contentwarnings)       | ❌    |             |             |
| `contentRights`      | [`place.stream.metadata.contentRights`](/lex-reference/place-stream-metadata-contentrights)           | ❌    |             |             |
| `distributionPolicy` | [`place.stream.metadata.distributionPolicy`](/lex-reference/place-stream-metadata-distributionpolicy) | ❌    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.metadata.configuration",
  "defs": {
    "main": {
      "type": "record",
      "description": "Default metadata record for livestream including content warnings, rights, and distribution policy",
      "key": "literal:self",
      "record": {
        "type": "object",
        "properties": {
          "contentWarnings": {
            "type": "ref",
            "ref": "place.stream.metadata.contentWarnings"
          },
          "contentRights": {
            "type": "ref",
            "ref": "place.stream.metadata.contentRights"
          },
          "distributionPolicy": {
            "type": "ref",
            "ref": "place.stream.metadata.distributionPolicy"
          }
        }
      }
    }
  }
}
```
