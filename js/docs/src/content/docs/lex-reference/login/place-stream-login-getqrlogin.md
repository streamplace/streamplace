---
title: place.stream.login.getQRLogin
description: Reference for the place.stream.login.getQRLogin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a QR login attempt: who is asking, for the approver, and its outcome, for the browser waiting on it. Once approved it carries the OAuth authorization response the browser completes its login with.

**Parameters:**

| Name | Type     | Req'd | Description | Constraints |
| ---- | -------- | ----- | ----------- | ----------- |
| `id` | `string` | ✅    |             |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** [`place.stream.login.defs#qrLogin`](/lex-reference/place-stream-login-defs#qrlogin)

**Possible Errors:**

- `NotFound`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.login.getQRLogin",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a QR login attempt: who is asking, for the approver, and its outcome, for the browser waiting on it. Once approved it carries the OAuth authorization response the browser completes its login with.",
      "parameters": {
        "type": "params",
        "required": ["id"],
        "properties": {
          "id": {
            "type": "string"
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
        }
      ]
    }
  }
}
```
