---
title: place.stream.vod.createDraft
description: Reference for the place.stream.vod.createDraft lexicon
---
**Lexicon Version:** 1

## Definitions

<a name="main"></a>
### `main`

**Type:** `procedure`

Create an empty draft VOD in the 'processing' state. The draft is the durable anchor for an upload: the client creates it first, then passes its URI to place.stream.media.createUpload so the upload's processing fills this draft. Lets the user start editing metadata while the upload runs, and supports re-upload (a failed upload leaves the draft intact; a new createUpload with the same draftUri re-points it).

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_
**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name | Type | Req'd  | Description | Constraints |
|------|------|----------|-------------|-------------|
| `uri` | `string` | ✅  | The ats:// URI of the newly created draft record. |  |

---

## Lexicon Source
```json
{
  "lexicon": 1,
  "id": "place.stream.vod.createDraft",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create an empty draft VOD in the 'processing' state. The draft is the durable anchor for an upload: the client creates it first, then passes its URI to place.stream.media.createUpload so the upload's processing fills this draft. Lets the user start editing metadata while the upload runs, and supports re-upload (a failed upload leaves the draft intact; a new createUpload with the same draftUri re-points it).",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "properties": {}
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": [
            "uri"
          ],
          "properties": {
            "uri": {
              "type": "string",
              "description": "The ats:// URI of the newly created draft record."
            }
          }
        }
      }
    }
  }
}
```
