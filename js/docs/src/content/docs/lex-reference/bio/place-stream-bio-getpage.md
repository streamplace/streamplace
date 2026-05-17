---
title: place.stream.bio.getPage
description: Reference for the place.stream.bio.getPage lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a user's bio page.

**Parameters:**

| Name   | Type     | Req'd | Description                             | Constraints             |
| ------ | -------- | ----- | --------------------------------------- | ----------------------- |
| `repo` | `string` | ✅    | The DID of the user whose bio to fetch. | Format: `at-identifier` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** [`place.stream.bio.page`](/lex-reference/place-stream-bio-page)

**Possible Errors:**

- `BioNotFound`: The user has no bio page.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.getPage",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a user's bio page.",
      "parameters": {
        "type": "params",
        "required": ["repo"],
        "properties": {
          "repo": {
            "type": "string",
            "format": "at-identifier",
            "description": "The DID of the user whose bio to fetch."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "ref",
          "ref": "place.stream.bio.page"
        }
      },
      "errors": [
        {
          "name": "BioNotFound",
          "description": "The user has no bio page."
        }
      ]
    }
  }
}
```
