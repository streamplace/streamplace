---
title: place.stream.vod.listDrafts
description: Reference for the place.stream.vod.listDrafts lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

List the authenticated user's draft VODs, newest first.

**Parameters:**

| Name     | Type      | Req'd | Description                                 | Constraints                           |
| -------- | --------- | ----- | ------------------------------------------- | ------------------------------------- |
| `limit`  | `integer` | ❌    | Number of drafts to return.                 | Min: 1<br/>Max: 100<br/>Default: `50` |
| `cursor` | `string`  | ❌    | Pagination cursor from a previous response. |                                       |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                                   | Req'd | Description                                                 | Constraints |
| -------- | ------------------------------------------------------------------------------------------------------ | ----- | ----------------------------------------------------------- | ----------- |
| `drafts` | Array of [`place.stream.vod.draftDefs#draftView`](/lex-reference/place-stream-vod-draftdefs#draftview) | ✅    |                                                             |             |
| `cursor` | `string`                                                                                               | ❌    | Pagination cursor for the next page, if more results exist. |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.listDrafts",
  "defs": {
    "main": {
      "type": "query",
      "description": "List the authenticated user's draft VODs, newest first.",
      "parameters": {
        "type": "params",
        "properties": {
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 50,
            "description": "Number of drafts to return."
          },
          "cursor": {
            "type": "string",
            "description": "Pagination cursor from a previous response."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["drafts"],
          "properties": {
            "drafts": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.vod.draftDefs#draftView"
              }
            },
            "cursor": {
              "type": "string",
              "description": "Pagination cursor for the next page, if more results exist."
            }
          }
        }
      }
    }
  }
}
```
