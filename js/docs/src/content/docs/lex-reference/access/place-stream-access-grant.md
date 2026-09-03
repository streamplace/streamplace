---
title: place.stream.access.grant
description: Reference for the place.stream.access.grant lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Grants one role to one account on a node. Lives in the node's access-control space (space type place.stream.access.control, skey self, authority = the broadcaster DID) and is authored by the admin who created it; until the atproto spaces implementation ships, the node stores it in statedb addressed by its at:// space URI.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                            | Req'd | Description                                 | Constraints                             |
| ----------- | ------------------------------------------------------------------------------- | ----- | ------------------------------------------- | --------------------------------------- |
| `subject`   | `string`                                                                        | ✅    | The account receiving the role.             | Format: `did`                           |
| `role`      | [`place.stream.access.defs#role`](/lex-reference/place-stream-access-defs#role) | ✅    |                                             |                                         |
| `createdAt` | `string`                                                                        | ✅    |                                             | Format: `datetime`                      |
| `note`      | `string`                                                                        | ❌    | Free-form reminder of why the grant exists. | Max Length: 1000<br/>Max Graphemes: 100 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.grant",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Grants one role to one account on a node. Lives in the node's access-control space (space type place.stream.access.control, skey self, authority = the broadcaster DID) and is authored by the admin who created it; until the atproto spaces implementation ships, the node stores it in statedb addressed by its at:// space URI.",
      "record": {
        "type": "object",
        "required": ["subject", "role", "createdAt"],
        "properties": {
          "subject": {
            "type": "string",
            "format": "did",
            "description": "The account receiving the role."
          },
          "role": {
            "type": "ref",
            "ref": "place.stream.access.defs#role"
          },
          "createdAt": {
            "type": "string",
            "format": "datetime"
          },
          "note": {
            "type": "string",
            "maxLength": 1000,
            "maxGraphemes": 100,
            "description": "Free-form reminder of why the grant exists."
          }
        }
      }
    }
  }
}
```
