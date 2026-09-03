---
title: place.stream.access.listGrants
description: Reference for the place.stream.access.listGrants lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

List every grant on this node, including grants seeded from the environment. Requires the admin role.

**Parameters:**

| Name   | Type     | Req'd | Description                      | Constraints |
| ------ | -------- | ----- | -------------------------------- | ----------- |
| `role` | `string` | ❌    | Only return grants of this role. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                               | Req'd | Description | Constraints |
| -------- | -------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `grants` | Array of [`place.stream.access.defs#grantView`](/lex-reference/place-stream-access-defs#grantview) | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`: The caller is not an admin.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.listGrants",
  "defs": {
    "main": {
      "type": "query",
      "description": "List every grant on this node, including grants seeded from the environment. Requires the admin role.",
      "parameters": {
        "type": "params",
        "properties": {
          "role": {
            "type": "string",
            "description": "Only return grants of this role."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["grants"],
          "properties": {
            "grants": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.access.defs#grantView"
              }
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The caller is not an admin."
        }
      ]
    }
  }
}
```
