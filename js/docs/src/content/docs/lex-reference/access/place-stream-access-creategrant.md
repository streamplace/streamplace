---
title: place.stream.access.createGrant
description: Reference for the place.stream.access.createGrant lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Grant a role to an account. Requires the admin role. Idempotent: granting a role an account already holds returns the existing grant.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                            | Req'd | Description                                                                                              | Constraints                             |
| --------- | ------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `subject` | `string`                                                                        | ✅    | The account to grant to, as a DID or a handle. Handles are resolved to DIDs before the grant is written. |                                         |
| `role`    | [`place.stream.access.defs#role`](/lex-reference/place-stream-access-defs#role) | ✅    |                                                                                                          |                                         |
| `note`    | `string`                                                                        | ❌    |                                                                                                          | Max Length: 1000<br/>Max Graphemes: 100 |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name    | Type                                                                                      | Req'd | Description | Constraints |
| ------- | ----------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `grant` | [`place.stream.access.defs#grantView`](/lex-reference/place-stream-access-defs#grantview) | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`: The caller is not an admin.
- `InvalidSubject`: The subject is neither a valid DID nor a resolvable handle.
- `InvalidRole`: The role is not one this node knows about.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.createGrant",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Grant a role to an account. Requires the admin role. Idempotent: granting a role an account already holds returns the existing grant.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["subject", "role"],
          "properties": {
            "subject": {
              "type": "string",
              "description": "The account to grant to, as a DID or a handle. Handles are resolved to DIDs before the grant is written."
            },
            "role": {
              "type": "ref",
              "ref": "place.stream.access.defs#role"
            },
            "note": {
              "type": "string",
              "maxLength": 1000,
              "maxGraphemes": 100
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["grant"],
          "properties": {
            "grant": {
              "type": "ref",
              "ref": "place.stream.access.defs#grantView"
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
          "name": "InvalidSubject",
          "description": "The subject is neither a valid DID nor a resolvable handle."
        },
        {
          "name": "InvalidRole",
          "description": "The role is not one this node knows about."
        }
      ]
    }
  }
}
```
