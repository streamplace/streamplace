---
title: place.stream.like
description: Reference for the place.stream.like lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record indicating a like on some other record (e.g. a video or a comment).

**Record Key:** `tid`

**Record Properties:**

| Name        | Type     | Req'd | Description                                                                               | Constraints        |
| ----------- | -------- | ----- | ----------------------------------------------------------------------------------------- | ------------------ |
| `subject`   | `string` | ✅    | AT-URI of the record being liked (e.g. a place.stream.video or place.stream.vod.comment). | Format: `at-uri`   |
| `createdAt` | `string` | ✅    | Client-declared timestamp when this like was created.                                     | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.like",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record indicating a like on some other record (e.g. a video or a comment).",
      "record": {
        "type": "object",
        "required": ["subject", "createdAt"],
        "properties": {
          "subject": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the record being liked (e.g. a place.stream.video or place.stream.vod.comment)."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this like was created."
          }
        }
      }
    }
  }
}
```
