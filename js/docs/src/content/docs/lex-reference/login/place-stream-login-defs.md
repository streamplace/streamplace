---
title: place.stream.login.defs
description: Reference for the place.stream.login.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="qrlogin"></a>

### `qrLogin`

**Type:** `object`

**Properties:**

| Name            | Type                                               | Req'd | Description                                      | Constraints                                              |
| --------------- | -------------------------------------------------- | ----- | ------------------------------------------------ | -------------------------------------------------------- |
| `id`            | `string`                                           | ✅    |                                                  |                                                          |
| `status`        | `string`                                           | ✅    |                                                  | Known Values: `pending`, `approved`, `denied`, `expired` |
| `userAgent`     | `string`                                           | ✅    | The User-Agent of the browser asking to sign in. |                                                          |
| `ip`            | `string`                                           | ✅    | The IP address the browser asked from.           |                                                          |
| `createdAt`     | `string`                                           | ✅    |                                                  | Format: `datetime`                                       |
| `expiresAt`     | `string`                                           | ✅    |                                                  | Format: `datetime`                                       |
| `authorization` | [`#authorizationResponse`](#authorizationresponse) | ❌    | Set once approved.                               |                                                          |

---

<a name="authorizationresponse"></a>

### `authorizationResponse`

**Type:** `object`

The OAuth authorization response for the browser's pushed authorization request, as its redirect would have carried it. Redeeming the code still takes the browser's PKCE verifier and DPoP key.

**Properties:**

| Name    | Type     | Req'd | Description | Constraints |
| ------- | -------- | ----- | ----------- | ----------- |
| `code`  | `string` | ✅    |             |             |
| `state` | `string` | ✅    |             |             |
| `iss`   | `string` | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.login.defs",
  "defs": {
    "qrLogin": {
      "type": "object",
      "required": ["id", "status", "userAgent", "ip", "createdAt", "expiresAt"],
      "properties": {
        "id": {
          "type": "string"
        },
        "status": {
          "type": "string",
          "knownValues": ["pending", "approved", "denied", "expired"]
        },
        "userAgent": {
          "type": "string",
          "description": "The User-Agent of the browser asking to sign in."
        },
        "ip": {
          "type": "string",
          "description": "The IP address the browser asked from."
        },
        "createdAt": {
          "type": "string",
          "format": "datetime"
        },
        "expiresAt": {
          "type": "string",
          "format": "datetime"
        },
        "authorization": {
          "type": "ref",
          "ref": "#authorizationResponse",
          "description": "Set once approved."
        }
      }
    },
    "authorizationResponse": {
      "type": "object",
      "description": "The OAuth authorization response for the browser's pushed authorization request, as its redirect would have carried it. Redeeming the code still takes the browser's PKCE verifier and DPoP key.",
      "required": ["code", "state", "iss"],
      "properties": {
        "code": {
          "type": "string"
        },
        "state": {
          "type": "string"
        },
        "iss": {
          "type": "string"
        }
      }
    }
  }
}
```
