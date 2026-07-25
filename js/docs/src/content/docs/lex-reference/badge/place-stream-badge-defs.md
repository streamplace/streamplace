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

| Name          | Type     | Req'd | Description                                                         | Constraints                                                                                                                                                                    |
| ------------- | -------- | ----- | ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `badgeType`   | `string` | ✅    |                                                                     | Known Values: `place.stream.badge.defs#mod`, `place.stream.badge.defs#streamer`, `place.stream.badge.defs#vip`, `place.stream.badge.defs#event`, `place.stream.badge.defs#bot` |
| `issuer`      | `string` | ✅    | DID of the badge issuer.                                            | Format: `did`                                                                                                                                                                  |
| `recipient`   | `string` | ✅    | DID of the badge recipient.                                         | Format: `did`                                                                                                                                                                  |
| `name`        | `string` | ❌    | Display name from the badge definition.                             |                                                                                                                                                                                |
| `description` | `string` | ❌    | Description from the badge definition.                              |                                                                                                                                                                                |
| `imageUrl`    | `string` | ❌    | Resolved image URL for the badge icon.                              | Format: `uri`                                                                                                                                                                  |
| `signature`   | `string` | ❌    | TODO: Cryptographic signature of the badge (of a place.stream.key). |                                                                                                                                                                                |

---

<a name="badgeissuanceview"></a>

### `badgeIssuanceView`

**Type:** `object`

A resolved view of a badge issuance, including def fields for display.

**Properties:**

| Name          | Type      | Req'd | Description                                                           | Constraints                                                                  |
| ------------- | --------- | ----- | --------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `issuanceUri` | `string`  | ✅    | AT URI of the place.stream.badge.issuance record.                     | Format: `at-uri`                                                             |
| `issuanceCid` | `string`  | ❌    | CID of the place.stream.badge.issuance record.                        |                                                                              |
| `badgeType`   | `string`  | ✅    |                                                                       | Known Values: `place.stream.badge.defs#vip`, `place.stream.badge.defs#event` |
| `name`        | `string`  | ❌    | Display name from the badge definition.                               |                                                                              |
| `description` | `string`  | ❌    | Description from the badge definition.                                |                                                                              |
| `imageUrl`    | `string`  | ❌    | Resolved image URL for the badge icon.                                | Format: `uri`                                                                |
| `issuer`      | `string`  | ✅    | DID of the badge issuer.                                              | Format: `did`                                                                |
| `selected`    | `boolean` | ❌    | Whether this badge is currently in the user's chat profile selection. |                                                                              |

---

<a name="badgeslot"></a>

### `badgeSlot`

**Type:** `object`

A display slot containing available issuance-based badges and which one (if any) is currently selected.

**Properties:**

| Name        | Type                                                | Req'd | Description                                        | Constraints |
| ----------- | --------------------------------------------------- | ----- | -------------------------------------------------- | ----------- |
| `selected`  | [`#badgeIssuanceView`](#badgeissuanceview)          | ❌    | The currently selected badge in this slot, if any. |             |
| `available` | Array of [`#badgeIssuanceView`](#badgeissuanceview) | ✅    | All badges available for this slot.                |             |

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

<a name="bot"></a>

### `bot`

**Type:** `token`

This user is a bot. Self-applied via place.stream.chat.profile selfLabels.

---

<a name="event"></a>

### `event`

**Type:** `token`

This user has won or earned a special event or contest badge.

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
            "place.stream.badge.defs#streamer",
            "place.stream.badge.defs#vip",
            "place.stream.badge.defs#event",
            "place.stream.badge.defs#bot"
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
        "name": {
          "type": "string",
          "description": "Display name from the badge definition."
        },
        "description": {
          "type": "string",
          "description": "Description from the badge definition."
        },
        "imageUrl": {
          "type": "string",
          "format": "uri",
          "description": "Resolved image URL for the badge icon."
        },
        "signature": {
          "type": "string",
          "description": "TODO: Cryptographic signature of the badge (of a place.stream.key)."
        }
      }
    },
    "badgeIssuanceView": {
      "type": "object",
      "required": ["issuanceUri", "badgeType", "issuer"],
      "description": "A resolved view of a badge issuance, including def fields for display.",
      "properties": {
        "issuanceUri": {
          "type": "string",
          "format": "at-uri",
          "description": "AT URI of the place.stream.badge.issuance record."
        },
        "issuanceCid": {
          "type": "string",
          "description": "CID of the place.stream.badge.issuance record."
        },
        "badgeType": {
          "type": "string",
          "knownValues": [
            "place.stream.badge.defs#vip",
            "place.stream.badge.defs#event"
          ]
        },
        "name": {
          "type": "string",
          "description": "Display name from the badge definition."
        },
        "description": {
          "type": "string",
          "description": "Description from the badge definition."
        },
        "imageUrl": {
          "type": "string",
          "format": "uri",
          "description": "Resolved image URL for the badge icon."
        },
        "issuer": {
          "type": "string",
          "format": "did",
          "description": "DID of the badge issuer."
        },
        "selected": {
          "type": "boolean",
          "description": "Whether this badge is currently in the user's chat profile selection."
        }
      }
    },
    "badgeSlot": {
      "type": "object",
      "required": ["available"],
      "description": "A display slot containing available issuance-based badges and which one (if any) is currently selected.",
      "properties": {
        "selected": {
          "type": "ref",
          "ref": "#badgeIssuanceView",
          "description": "The currently selected badge in this slot, if any."
        },
        "available": {
          "type": "array",
          "description": "All badges available for this slot.",
          "items": {
            "type": "ref",
            "ref": "#badgeIssuanceView"
          }
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
    },
    "bot": {
      "type": "token",
      "description": "This user is a bot. Self-applied via place.stream.chat.profile selfLabels."
    },
    "event": {
      "type": "token",
      "description": "This user has won or earned a special event or contest badge."
    }
  }
}
```
