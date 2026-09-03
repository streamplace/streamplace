---
title: place.stream.access.deleteGrant
description: Reference for the place.stream.access.deleteGrant lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Revoke a grant by its space URI. Requires the admin role. Grants seeded from the environment have no URI and cannot be revoked here.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                                                                                         | Constraints |
| ----- | -------- | ----- | --------------------------------------------------------------------------------------------------- | ----------- |
| `uri` | `string` | ✅    | (A space URI; not validated as a classic at-uri because the space form is newer than that grammar.) |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description | Constraints |
| --------- | --------- | ----- | ----------- | ----------- |
| `success` | `boolean` | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`: The caller is not an admin.
- `NotFound`: No grant with that URI.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.deleteGrant",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Revoke a grant by its space URI. Requires the admin role. Grants seeded from the environment have no URI and cannot be revoked here.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri"],
          "properties": {
            "uri": {
              "type": "string",
              "description": "(A space URI; not validated as a classic at-uri because the space form is newer than that grammar.)"
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["success"],
          "properties": {
            "success": {
              "type": "boolean"
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The caller is not an admin."
        },
        {
          "name": "NotFound",
          "description": "No grant with that URI."
        }
      ]
    }
  }
}
```
