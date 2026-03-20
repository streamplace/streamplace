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

| Name            | Type     | Req'd | Description                                                                       | Constraints        |
| --------------- | -------- | ----- | --------------------------------------------------------------------------------- | ------------------ |
| `pinnedMessage` | `string` | ✅    | AT-URI of the pinned chat message.                                                | Format: `at-uri`   |
| `createdAt`     | `string` | ✅    | When this pin was created.                                                        | Format: `datetime` |
| `expiresAt`     | `string` | ❌    | Optional expiration time. If set, the pin is considered inactive after this time. | Format: `datetime` |

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
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "When this pin was created."
          },
          "expiresAt": {
            "type": "string",
            "format": "datetime",
            "description": "Optional expiration time. If set, the pin is considered inactive after this time."
          }
        }
      }
    }
  }
}
```
