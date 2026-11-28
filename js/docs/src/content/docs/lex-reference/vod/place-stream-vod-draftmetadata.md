---
title: place.stream.vod.draftMetadata
description: Reference for the place.stream.vod.draftMetadata lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

User-editable VOD draft metadata. Stored only in the user's personal space, never in their public repo.

**Record Key:** `tid`

**Record Properties:**

| Name                | Type                                                                                                                                                                                                            | Req'd | Description | Constraints                                   |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | --------------------------------------------- |
| `title`             | `string`                                                                                                                                                                                                        | ✅    |             | Max Length: 1400<br/>Max Graphemes: 140       |
| `description`       | `string`                                                                                                                                                                                                        | ❌    |             | Max Length: 50000<br/>Max Graphemes: 5000     |
| `descriptionFacets` | Array of [`place.stream.richtext.videoFacet`](/lex-reference/place-stream-richtext-videofacet)                                                                                                                  | ❌    |             |                                               |
| `thumb`             | `blob`                                                                                                                                                                                                          | ❌    |             | Accept: `image/*`<br/>Max Size: 1000000 bytes |
| `activity`          | Union of:<br/>&nbsp;&nbsp;[`place.stream.defs#activityGame`](/lex-reference/place-stream-defs#activitygame)<br/>&nbsp;&nbsp;[`place.stream.defs#activityLabel`](/lex-reference/place-stream-defs#activitylabel) | ❌    |             |                                               |
| `tags`              | Array of `string`                                                                                                                                                                                               | ❌    |             | Max Items: 10                                 |
| `contentWarnings`   | [`place.stream.metadata.contentWarnings`](/lex-reference/place-stream-metadata-contentwarnings)                                                                                                                 | ❌    |             |                                               |
| `contentRights`     | [`place.stream.metadata.contentRights`](/lex-reference/place-stream-metadata-contentrights)                                                                                                                     | ❌    |             |                                               |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.draftMetadata",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "User-editable VOD draft metadata. Stored only in the user's personal space, never in their public repo.",
      "record": {
        "type": "object",
        "required": ["title"],
        "properties": {
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140
          },
          "description": {
            "type": "string",
            "maxLength": 50000,
            "maxGraphemes": 5000
          },
          "descriptionFacets": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "place.stream.richtext.videoFacet"
            }
          },
          "thumb": {
            "type": "blob",
            "accept": ["image/*"],
            "maxSize": 1000000
          },
          "activity": {
            "type": "union",
            "refs": [
              "place.stream.defs#activityGame",
              "place.stream.defs#activityLabel"
            ]
          },
          "tags": {
            "type": "array",
            "maxLength": 10,
            "items": {
              "type": "string",
              "maxLength": 640,
              "maxGraphemes": 64
            }
          },
          "contentWarnings": {
            "type": "ref",
            "ref": "place.stream.metadata.contentWarnings"
          },
          "contentRights": {
            "type": "ref",
            "ref": "place.stream.metadata.contentRights"
          }
        }
      }
    }
  }
}
```
