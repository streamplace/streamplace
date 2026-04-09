---
title: place.stream.chat.gate
description: Reference for the place.stream.chat.gate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record defining a single gated chat message.

**Record Key:** `tid`

**Record Properties:**

| Name            | Type     | Req'd | Description                     | Constraints      |
| --------------- | -------- | ----- | ------------------------------- | ---------------- |
| `hiddenMessage` | `string` | ✅    | URI of the hidden chat message. | Format: `at-uri` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.chat.gate",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record defining a single gated chat message.",
      "record": {
        "type": "object",
        "required": ["hiddenMessage"],
        "properties": {
          "hiddenMessage": {
            "type": "string",
            "format": "at-uri",
            "description": "URI of the hidden chat message."
          }
        }
      }
    }
  }
}
```
