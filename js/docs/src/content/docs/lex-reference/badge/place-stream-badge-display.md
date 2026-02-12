---
title: place.stream.badge.display
description: Reference for the place.stream.badge.display lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record issuing a badge to a user.

**Record Properties:**

| Name     | Type                                          | Req'd | Description                                                                                                             | Constraints  |
| -------- | --------------------------------------------- | ----- | ----------------------------------------------------------------------------------------------------------------------- | ------------ |
| `badges` | Array of [`#badgeSelection`](#badgeselection) | ✅    | Up to 3 badge tokens to display with the message. First badge is server-controlled, remaining badges are user-settable. | Max Items: 3 |

---

<a name="badgeselection"></a>

### `badgeSelection`

**Type:** `object`

A badge selected for display. May be a full badgeView from the server, or a token representing a badge type that the client can look up for display info.

**Properties:**

| Name        | Type     | Req'd | Description                                                                                                                         | Constraints                                                                |
| ----------- | -------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| `badgeType` | `string` | ✅    |                                                                                                                                     | Known Values: `place.stream.badge.defs#mod`, `place.stream.badge.defs#vip` |
| `issuance`  | `string` | ❌    | URI of the badge issuance record (place.stream.badge.issuance) that represents this badge. Required if badgeType is not recognized. | Format: `at-uri`                                                           |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.display",
  "defs": {
    "main": {
      "type": "record",
      "description": "Record issuing a badge to a user.",
      "record": {
        "type": "object",
        "required": ["badges"],
        "properties": {
          "badges": {
            "type": "array",
            "description": "Up to 3 badge tokens to display with the message. First badge is server-controlled, remaining badges are user-settable.",
            "maxLength": 3,
            "items": {
              "type": "ref",
              "ref": "#badgeSelection"
            }
          }
        }
      }
    },
    "badgeSelection": {
      "type": "object",
      "description": "A badge selected for display. May be a full badgeView from the server, or a token representing a badge type that the client can look up for display info.",
      "required": ["badgeType"],
      "properties": {
        "badgeType": {
          "type": "string",
          "knownValues": [
            "place.stream.badge.defs#mod",
            "place.stream.badge.defs#vip"
          ]
        },
        "issuance": {
          "type": "string",
          "format": "at-uri",
          "description": "URI of the badge issuance record (place.stream.badge.issuance) that represents this badge. Required if badgeType is not recognized."
        }
      }
    }
  }
}
```
