---
title: place.stream.badge.issuance
description: Reference for the place.stream.badge.issuance lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record issuing a badge to a user.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type     | Req'd | Description                                                         | Constraints                                 |
| ----------- | -------- | ----- | ------------------------------------------------------------------- | ------------------------------------------- |
| `badgeType` | `string` | ✅    |                                                                     | Known Values: `place.stream.badge.defs#vip` |
| `recipient` | `string` | ✅    | DID of the badge recipient.                                         | Format: `did`                               |
| `signature` | `string` | ✅    | TODO: Cryptographic signature of the badge (of a place.stream.key). |                                             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.issuance",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record issuing a badge to a user.",
      "record": {
        "type": "object",
        "required": ["badgeType", "recipient", "signature"],
        "properties": {
          "badgeType": {
            "type": "string",
            "knownValues": ["place.stream.badge.defs#vip"]
          },
          "recipient": {
            "type": "string",
            "format": "did",
            "description": "DID of the badge recipient."
          },
          "signature": {
            "type": "string",
            "description": "TODO: Cryptographic signature of the badge (of a place.stream.key)."
          }
        }
      }
    }
  }
}
```
