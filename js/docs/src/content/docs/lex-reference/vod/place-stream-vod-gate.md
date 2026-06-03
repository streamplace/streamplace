---
title: place.stream.vod.gate
description: Reference for the place.stream.vod.gate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record defining a single gated VOD comment.

**Record Key:** `tid`

**Record Properties:**

| Name            | Type     | Req'd | Description                    | Constraints      |
| --------------- | -------- | ----- | ------------------------------ | ---------------- |
| `hiddenComment` | `string` | ✅    | URI of the hidden VOD comment. | Format: `at-uri` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.gate",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record defining a single gated VOD comment.",
      "record": {
        "type": "object",
        "required": ["hiddenComment"],
        "properties": {
          "hiddenComment": {
            "type": "string",
            "format": "at-uri",
            "description": "URI of the hidden VOD comment."
          }
        }
      }
    }
  }
}
```
