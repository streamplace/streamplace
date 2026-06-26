---
title: place.stream.vod.deleteDraft
description: Reference for the place.stream.vod.deleteDraft lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Discard a draft VOD. The processed content blob is not deleted (it may be referenced elsewhere). Only the draft record is removed.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                             | Constraints |
| ----- | -------- | ----- | --------------------------------------- | ----------- |
| `uri` | `string` | ✅    | The ats:// URI of the draft to discard. |             |

**Possible Errors:**

- `NotFound`: No draft exists with the given URI for the authenticated user.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.deleteDraft",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Discard a draft VOD. The processed content blob is not deleted (it may be referenced elsewhere). Only the draft record is removed.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri"],
          "properties": {
            "uri": {
              "type": "string",
              "description": "The ats:// URI of the draft to discard."
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
