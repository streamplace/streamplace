---
title: place.stream.login.completeQRLogin
description: Reference for the place.stream.login.completeQRLogin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Approve or deny a QR login attempt. Requires an OAuth session on this node: approving signs the waiting browser in as the caller, sharing the caller's session with their PDS.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description | Constraints |
| --------- | --------- | ----- | ----------- | ----------- |
| `id`      | `string`  | ✅    |             |             |
| `approve` | `boolean` | ✅    |             |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** [`place.stream.login.defs#qrLogin`](/lex-reference/place-stream-login-defs#qrlogin)

**Possible Errors:**

- `NotFound`
- `AlreadyCompleted`: The attempt was already approved or denied, or has expired.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.login.completeQRLogin",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Approve or deny a QR login attempt. Requires an OAuth session on this node: approving signs the waiting browser in as the caller, sharing the caller's session with their PDS.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["id", "approve"],
          "properties": {
            "id": {
              "type": "string"
            },
            "approve": {
              "type": "boolean"
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "ref",
          "ref": "place.stream.login.defs#qrLogin"
        }
      },
      "errors": [
        {
          "name": "NotFound"
        },
        {
          "name": "AlreadyCompleted",
          "description": "The attempt was already approved or denied, or has expired."
        }
      ]
    }
  }
}
```
