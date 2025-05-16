---
title: place.stream.account.login
description: Reference for the place.stream.account.login lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Get a redirect URL for the login flow.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type     | Req'd | Description                                | Constraints |
| ------------- | -------- | ----- | ------------------------------------------ | ----------- |
| `handleOrDID` | `string` | ✅    | The handle or DID of the account to login. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:**
[`place.stream.account.defs#loginResponse`](/lex-reference/place-stream-account-defs#loginresponse)

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.account.login",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Get a redirect URL for the login flow.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["handleOrDID"],
          "properties": {
            "handleOrDID": {
              "type": "string",
              "description": "The handle or DID of the account to login."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "ref",
          "ref": "place.stream.account.defs#loginResponse"
        }
      }
    }
  }
}
```
