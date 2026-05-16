---
title: place.stream.beta.invite
description: Reference for the place.stream.beta.invite lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Grants a single account access to a named beta feature on this network. Records live in a project-operated repo (the streamplace node's `--beta-invite-did`); operators trust invites only from that one account, by configuration. Each (did, feature) pair gets its own record so individual features can be revoked independently by deleting the corresponding record.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type     | Req'd | Description                                                | Constraints        |
| ----------- | -------- | ----- | ---------------------------------------------------------- | ------------------ |
| `did`       | `string` | ✅    | Account being granted access to the named beta feature.    | Format: `did`      |
| `feature`   | `string` | ✅    | Identifier of the beta feature being granted (e.g. "vod"). |                    |
| `createdAt` | `string` | ✅    | Client-declared timestamp when this invite was issued.     | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.beta.invite",
  "defs": {
    "main": {
      "type": "record",
      "description": "Grants a single account access to a named beta feature on this network. Records live in a project-operated repo (the streamplace node's `--beta-invite-did`); operators trust invites only from that one account, by configuration. Each (did, feature) pair gets its own record so individual features can be revoked independently by deleting the corresponding record.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["did", "feature", "createdAt"],
        "properties": {
          "did": {
            "type": "string",
            "format": "did",
            "description": "Account being granted access to the named beta feature."
          },
          "feature": {
            "type": "string",
            "description": "Identifier of the beta feature being granted (e.g. \"vod\")."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this invite was issued."
          }
        }
      }
    }
  }
}
```
