---
title: place.stream.chat.pinnedRecord
description: Reference for the place.stream.chat.pinnedRecord lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record pinning a chat message for prominent display.

**Record Key:** `tid`

**Record Properties:**

| Name            | Type     | Req'd | Description                                                                                                                                                                                                                                                                             | Constraints        |
| --------------- | -------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `pinnedMessage` | `string` | ✅    | AT-URI of the pinned chat message.                                                                                                                                                                                                                                                      | Format: `at-uri`   |
| `pinnedBy`      | `string` | ❌    | DID of the user who pinned the message.                                                                                                                                                                                                                                                 | Format: `did`      |
| `createdAt`     | `string` | ✅    | When this pin was created.                                                                                                                                                                                                                                                              | Format: `datetime` |
| `expiresAt`     | `string` | ❌    | Optional expiration time. If 'livestream' is not set, the pin is considered inactive after this time.                                                                                                                                                                                   | Format: `datetime` |
| `livestream`    | `string` | ❌    | AT-URI of the place.stream.livestream record this pin is scoped to. If set, the pin is active only while that livestream exists and has not ended, and takes precedence over 'expiresAt'. If neither 'livestream' nor 'expiresAt' is set, the pin stays active until manually unpinned. | Format: `at-uri`   |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.chat.pinnedRecord",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record pinning a chat message for prominent display.",
      "record": {
        "type": "object",
        "required": ["pinnedMessage", "createdAt"],
        "properties": {
          "pinnedMessage": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the pinned chat message."
          },
          "pinnedBy": {
            "type": "string",
            "format": "did",
            "description": "DID of the user who pinned the message."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "When this pin was created."
          },
          "expiresAt": {
            "type": "string",
            "format": "datetime",
            "description": "Optional expiration time. If 'livestream' is not set, the pin is considered inactive after this time."
          },
          "livestream": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the place.stream.livestream record this pin is scoped to. If set, the pin is active only while that livestream exists and has not ended, and takes precedence over 'expiresAt'. If neither 'livestream' nor 'expiresAt' is set, the pin stays active until manually unpinned."
          }
        }
      }
    }
  }
}
```
