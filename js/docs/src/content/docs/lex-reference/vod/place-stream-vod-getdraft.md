---
title: place.stream.vod.getDraft
description: Reference for the place.stream.vod.getDraft lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a single draft VOD by its ats:// URI. Only accessible by the draft's author.

**Parameters:**

| Name  | Type     | Req'd | Description                         | Constraints |
| ----- | -------- | ----- | ----------------------------------- | ----------- |
| `uri` | `string` | ✅    | The ats:// URI of the draft record. |             |

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
  "id": "place.stream.vod.getDraft",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a single draft VOD by its ats:// URI. Only accessible by the draft's author.",
      "parameters": {
        "type": "params",
        "required": ["uri"],
        "properties": {
          "uri": {
            "type": "string",
            "description": "The ats:// URI of the draft record."
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
