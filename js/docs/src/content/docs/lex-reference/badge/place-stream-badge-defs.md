---
title: place.stream.badge.defs
description: Reference for the place.stream.badge.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="badgeview"></a>

### `badgeView`

**Type:** `object`

View of a badge record, with fields resolved for display. If the DID in issuer is not the current streamplace node, the signature field shall be required.

**Properties:**

| Name        | Type     | Req'd | Description                                                         | Constraints                                                                     |
| ----------- | -------- | ----- | ------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `badgeType` | `string` | ✅    |                                                                     | Known Values: `place.stream.badge.defs#mod`, `place.stream.badge.defs#streamer` |
| `issuer`    | `string` | ✅    | DID of the badge issuer.                                            | Format: `did`                                                                   |
| `recipient` | `string` | ✅    | DID of the badge recipient.                                         | Format: `did`                                                                   |
| `signature` | `string` | ❌    | TODO: Cryptographic signature of the badge (of a place.stream.key). |                                                                                 |

---

<a name="mod"></a>

### `mod`

**Type:** `token`

This user is a moderator. Displayed with a sword icon.

---

<a name="streamer"></a>

### `streamer`

**Type:** `token`

This user is the streamer. Displayed with a star icon.

---

<a name="vip"></a>

### `vip`

**Type:** `token`

This user is a very important person.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.defs",
  "defs": {
    "badgeView": {
      "type": "object",
      "required": ["badgeType", "issuer", "recipient"],
      "description": "View of a badge record, with fields resolved for display. If the DID in issuer is not the current streamplace node, the signature field shall be required.",
      "properties": {
        "badgeType": {
          "type": "string",
          "knownValues": [
            "place.stream.badge.defs#mod",
            "place.stream.badge.defs#streamer"
          ]
        },
        "issuer": {
          "type": "string",
          "format": "did",
          "description": "DID of the badge issuer."
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
    },
    "mod": {
      "type": "token",
      "description": "This user is a moderator. Displayed with a sword icon."
    },
    "streamer": {
      "type": "token",
      "description": "This user is the streamer. Displayed with a star icon."
    },
    "vip": {
      "type": "token",
      "description": "This user is a very important person."
    }
  }
}
```
