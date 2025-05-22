---
title: place.stream.key
description: Reference for the place.stream.key lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record linking an atproto identity with a stream signing key

**Record Key:** `tid`

**Record Properties:**

| Name         | Type     | Req'd | Description                                          | Constraints                       |
| ------------ | -------- | ----- | ---------------------------------------------------- | --------------------------------- |
| `signingKey` | `string` | ✅    | The did:key signing key for the stream.              | Min Length: 57<br/>Max Length: 57 |
| `createdAt`  | `string` | ✅    | Client-declared timestamp when this key was created. | Format: `datetime`                |
| `createdBy`  | `string` | ❌    | The name of the client that created this key.        |                                   |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.key",
  "defs": {
    "main": {
      "type": "record",
      "description": "Record linking an atproto identity with a stream signing key",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["signingKey", "createdAt"],
        "properties": {
          "signingKey": {
            "type": "string",
            "maxLength": 57,
            "minLength": 57,
            "description": "The did:key signing key for the stream."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this key was created."
          },
          "createdBy": {
            "type": "string",
            "description": "The name of the client that created this key."
          }
        }
      }
    }
  }
}
```
