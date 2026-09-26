---
title: place.stream.branding.listDomains
description: Reference for the place.stream.branding.listDomains lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Custom domains on this node: every one for an admin, the caller's own otherwise.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                                                     | Req'd | Description | Constraints |
| --------- | -------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `domains` | Array of [`place.stream.branding.defs#domainView`](/lex-reference/place-stream-branding-defs#domainview) | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.listDomains",
  "defs": {
    "main": {
      "type": "query",
      "description": "Custom domains on this node: every one for an admin, the caller's own otherwise.",
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["domains"],
          "properties": {
            "domains": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.branding.defs#domainView"
              }
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized"
        }
      ]
    }
  }
}
```
