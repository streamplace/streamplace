---
title: place.stream.bio.page
description: Reference for the place.stream.bio.page lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A Streamplace user's bio: a long-form 'about' description, pinned social links, and a customizable layout of blocks.

**Record Key:** `literal:self`

**Record Properties:**

| Name           | Type                                                                                                          | Req'd | Description                                                                                                                                                                 | Constraints        |
| -------------- | ------------------------------------------------------------------------------------------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `description`  | [`#description`](#description)                                                                                | ❌    | Long-form 'about' text shown in the bio header. Distinct from the user's bsky profile description, which is shorter.                                                        |                    |
| `socials`      | Array of [`place.stream.bio.defs#social`](/lex-reference/place-stream-bio-defs#social)                        | ❌    | Pinned social/platform links shown in the bio header.                                                                                                                       | Max Items: 16      |
| `layout`       | Union of:<br/>&nbsp;&nbsp;[`place.stream.bio.layouts.panels`](/lex-reference/place-stream-bio-layouts-panels) | ❌    | How the customizable body of the bio is arranged.                                                                                                                           |                    |
| `importedFrom` | `string`                                                                                                      | ❌    | Optional reference to an external authoring source (e.g. a pub.leaflet.document) this bio was imported/translated from. Clients may use this to offer a 're-import' action. | Format: `at-uri`   |
| `createdAt`    | `string`                                                                                                      | ✅    |                                                                                                                                                                             | Format: `datetime` |
| `updatedAt`    | `string`                                                                                                      | ❌    |                                                                                                                                                                             | Format: `datetime` |

---

<a name="description"></a>

### `description`

**Type:** `object`

Long-form rich text describing the user.

**Properties:**

| Name        | Type                                                                                 | Req'd | Description                                                | Constraints                               |
| ----------- | ------------------------------------------------------------------------------------ | ----- | ---------------------------------------------------------- | ----------------------------------------- |
| `plaintext` | `string`                                                                             | ✅    |                                                            | Max Length: 30000<br/>Max Graphemes: 3000 |
| `facets`    | Array of [`place.stream.richtext.facet`](/lex-reference/place-stream-richtext-facet) | ❌    | Annotations of the description text (mentions, URLs, etc). |                                           |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.page",
  "defs": {
    "main": {
      "type": "record",
      "description": "A Streamplace user's bio: a long-form 'about' description, pinned social links, and a customizable layout of blocks.",
      "key": "literal:self",
      "record": {
        "type": "object",
        "required": ["createdAt"],
        "properties": {
          "description": {
            "type": "ref",
            "ref": "#description",
            "description": "Long-form 'about' text shown in the bio header. Distinct from the user's bsky profile description, which is shorter."
          },
          "socials": {
            "type": "array",
            "description": "Pinned social/platform links shown in the bio header.",
            "maxLength": 16,
            "items": {
              "type": "ref",
              "ref": "place.stream.bio.defs#social"
            }
          },
          "layout": {
            "type": "union",
            "description": "How the customizable body of the bio is arranged.",
            "refs": ["place.stream.bio.layouts.panels"]
          },
          "importedFrom": {
            "type": "string",
            "format": "at-uri",
            "description": "Optional reference to an external authoring source (e.g. a pub.leaflet.document) this bio was imported/translated from. Clients may use this to offer a 're-import' action."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime"
          },
          "updatedAt": {
            "type": "string",
            "format": "datetime"
          }
        }
      }
    },
    "description": {
      "type": "object",
      "description": "Long-form rich text describing the user.",
      "required": ["plaintext"],
      "properties": {
        "plaintext": {
          "type": "string",
          "maxLength": 30000,
          "maxGraphemes": 3000
        },
        "facets": {
          "type": "array",
          "description": "Annotations of the description text (mentions, URLs, etc).",
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
