---
title: place.stream.vod.updateDraft
description: Reference for the place.stream.vod.updateDraft lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Update a draft VOD's editable metadata. Server-authoritative fields (source, durationMs, status) are preserved. Editing is allowed in any status, so the user can pre-fill metadata while processing. All fields except uri are optional (partial update).

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name                | Type                                                                                                                                                                                                            | Req'd | Description                            | Constraints                                   |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------- | --------------------------------------------- |
| `uri`               | `string`                                                                                                                                                                                                        | ✅    | The ats:// URI of the draft to update. |                                               |
| `title`             | `string`                                                                                                                                                                                                        | ❌    |                                        | Max Length: 1400<br/>Max Graphemes: 140       |
| `description`       | `string`                                                                                                                                                                                                        | ❌    |                                        | Max Length: 50000<br/>Max Graphemes: 5000     |
| `descriptionFacets` | Array of [`place.stream.richtext.videoFacet`](/lex-reference/place-stream-richtext-videofacet)                                                                                                                  | ❌    |                                        |                                               |
| `thumb`             | `blob`                                                                                                                                                                                                          | ❌    |                                        | Accept: `image/*`<br/>Max Size: 1000000 bytes |
| `activity`          | Union of:<br/>&nbsp;&nbsp;[`place.stream.defs#activityGame`](/lex-reference/place-stream-defs#activitygame)<br/>&nbsp;&nbsp;[`place.stream.defs#activityLabel`](/lex-reference/place-stream-defs#activitylabel) | ❌    |                                        |                                               |
| `tags`              | Array of `string`                                                                                                                                                                                               | ❌    |                                        | Max Items: 10                                 |
| `contentWarnings`   | [`place.stream.metadata.contentWarnings`](/lex-reference/place-stream-metadata-contentwarnings)                                                                                                                 | ❌    |                                        |                                               |
| `contentRights`     | [`place.stream.metadata.contentRights`](/lex-reference/place-stream-metadata-contentrights)                                                                                                                     | ❌    |                                        |                                               |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name    | Type                                                                                          | Req'd | Description | Constraints |
| ------- | --------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `draft` | [`place.stream.vod.draftDefs#draftView`](/lex-reference/place-stream-vod-draftdefs#draftview) | ✅    |             |             |

**Possible Errors:**

- `NotFound`: No draft exists with the given URI for the authenticated user.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.updateDraft",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Update a draft VOD's editable metadata. Server-authoritative fields (source, durationMs, status) are preserved. Editing is allowed in any status, so the user can pre-fill metadata while processing. All fields except uri are optional (partial update).",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri"],
          "properties": {
            "uri": {
              "type": "string",
              "description": "The ats:// URI of the draft to update."
            },
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
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["draft"],
          "properties": {
            "draft": {
              "type": "ref",
              "ref": "place.stream.vod.draftDefs#draftView"
            }
          }
        }
      },
      "errors": [
        {
          "name": "NotFound",
          "description": "No draft exists with the given URI for the authenticated user."
        }
      ]
    }
  }
}
```
