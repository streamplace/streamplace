---
title: place.stream.access.updatePolicy
description: Reference for the place.stream.access.updatePolicy lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Set the mode of one or more roles. Requires the admin role. Roles not mentioned keep their current mode. The admin role cannot be changed: it is always allowlist.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name    | Type                                                                                             | Req'd | Description | Constraints |
| ------- | ------------------------------------------------------------------------------------------------ | ----- | ----------- | ----------- |
| `roles` | Array of [`place.stream.access.defs#roleMode`](/lex-reference/place-stream-access-defs#rolemode) | ✅    |             |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                        | Req'd | Description | Constraints |
| -------- | ------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `policy` | [`place.stream.access.defs#policyView`](/lex-reference/place-stream-access-defs#policyview) | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`: The caller is not an admin.
- `InvalidRole`: A role or mode is not one this node knows about.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.updatePolicy",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Set the mode of one or more roles. Requires the admin role. Roles not mentioned keep their current mode. The admin role cannot be changed: it is always allowlist.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["roles"],
          "properties": {
            "roles": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.access.defs#roleMode"
              }
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["policy"],
          "properties": {
            "policy": {
              "type": "ref",
              "ref": "place.stream.access.defs#policyView"
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
          "name": "InvalidRole",
          "description": "A role or mode is not one this node knows about."
        }
      ]
    }
  }
}
```
