---
title: place.stream.bio.blocks.schedule
description: Reference for the place.stream.bio.blocks.schedule lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

A weekly streaming schedule, rendered as a grid.

**Properties:**

| Name       | Type                      | Req'd | Description                                                                    | Constraints    |
| ---------- | ------------------------- | ----- | ------------------------------------------------------------------------------ | -------------- |
| `timezone` | `string`                  | ✅    | IANA timezone name in which slot times are expressed, e.g. 'America/New_York'. | Max Length: 64 |
| `slots`    | Array of [`#slot`](#slot) | ✅    |                                                                                | Max Items: 21  |

---

<a name="slot"></a>

### `slot`

**Type:** `object`

**Properties:**

| Name        | Type     | Req'd | Description                                                                                 | Constraints                                           |
| ----------- | -------- | ----- | ------------------------------------------------------------------------------------------- | ----------------------------------------------------- |
| `dayOfWeek` | `string` | ✅    |                                                                                             | Enum: `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun` |
| `startTime` | `string` | ✅    | Start time in 24-hour HH:MM format, in the schedule's timezone.                             |                                                       |
| `endTime`   | `string` | ❌    | Optional end time in 24-hour HH:MM format. Slots with no endTime are treated as open-ended. |                                                       |
| `title`     | `string` | ❌    | Optional label for this slot, e.g. 'IRL walks' or 'Subathon'.                               | Max Length: 1000<br/>Max Graphemes: 100               |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.schedule",
  "defs": {
    "main": {
      "type": "object",
      "description": "A weekly streaming schedule, rendered as a grid.",
      "required": ["timezone", "slots"],
      "properties": {
        "timezone": {
          "type": "string",
          "description": "IANA timezone name in which slot times are expressed, e.g. 'America/New_York'.",
          "maxLength": 64
        },
        "slots": {
          "type": "array",
          "maxLength": 21,
          "items": {
            "type": "ref",
            "ref": "#slot"
          }
        }
      }
    },
    "slot": {
      "type": "object",
      "required": ["dayOfWeek", "startTime"],
      "properties": {
        "dayOfWeek": {
          "type": "string",
          "enum": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
        },
        "startTime": {
          "type": "string",
          "description": "Start time in 24-hour HH:MM format, in the schedule's timezone."
        },
        "endTime": {
          "type": "string",
          "description": "Optional end time in 24-hour HH:MM format. Slots with no endTime are treated as open-ended."
        },
        "title": {
          "type": "string",
          "description": "Optional label for this slot, e.g. 'IRL walks' or 'Subathon'.",
          "maxLength": 1000,
          "maxGraphemes": 100
        }
      }
    }
  }
}
```
