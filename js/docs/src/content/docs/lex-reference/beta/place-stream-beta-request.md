---
title: place.stream.beta.request
description: Reference for the place.stream.beta.request lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Requests access to a named beta feature on this network. Published in the requester's own repo, so the requesting account is the record's authority — there is no separate subject field. Operators index these to surface who is waiting for access; granting access is a separate place.stream.beta.invite record issued by the operator-trusted issuer. Each (requester, feature) pair gets its own record.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type     | Req'd | Description                                                                                                           | Constraints        |
| ----------- | -------- | ----- | --------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `feature`   | `string` | ✅    | Identifier of the beta feature being requested (e.g. "vod"). Mirrors the `feature` field on place.stream.beta.invite. |                    |
| `createdAt` | `string` | ✅    | Client-declared timestamp when this request was made.                                                                 | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.beta.request",
  "defs": {
    "main": {
      "type": "record",
      "description": "Requests access to a named beta feature on this network. Published in the requester's own repo, so the requesting account is the record's authority — there is no separate subject field. Operators index these to surface who is waiting for access; granting access is a separate place.stream.beta.invite record issued by the operator-trusted issuer. Each (requester, feature) pair gets its own record.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["feature", "createdAt"],
        "properties": {
          "feature": {
            "type": "string",
            "description": "Identifier of the beta feature being requested (e.g. \"vod\"). Mirrors the `feature` field on place.stream.beta.invite."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this request was made."
          }
        }
      }
    }
  }
}
```
