---
title: place.stream.live.recommendations
description: Reference for the place.stream.live.recommendations lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A list of recommended streamers, in order of preference

**Record Key:** `literal:self`

**Record Properties:**

| Name        | Type              | Req'd | Description                                           | Constraints                   |
| ----------- | ----------------- | ----- | ----------------------------------------------------- | ----------------------------- |
| `streamers` | Array of `string` | ✅    | Ordered list of recommended streamer DIDs             | Min Items: 0<br/>Max Items: 8 |
| `createdAt` | `string`          | ✅    | Client-declared timestamp when this list was created. | Format: `datetime`            |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.recommendations",
  "defs": {
    "main": {
      "type": "record",
      "description": "A list of recommended streamers, in order of preference",
      "key": "literal:self",
      "record": {
        "type": "object",
        "required": ["streamers", "createdAt"],
        "properties": {
          "streamers": {
            "type": "array",
            "description": "Ordered list of recommended streamer DIDs",
            "items": {
              "type": "string",
              "format": "did"
            },
            "maxLength": 8,
            "minLength": 0
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this list was created."
          }
        }
      }
    }
  }
}
```
