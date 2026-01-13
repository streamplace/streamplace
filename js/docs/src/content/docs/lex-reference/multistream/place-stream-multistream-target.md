---
title: place.stream.multistream.target
description: Reference for the place.stream.multistream.target lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

An external server for rebroadcasting a Streamplace stream

**Record Key:** `tid`

**Record Properties:**

| Name        | Type      | Req'd | Description                                       | Constraints        |
| ----------- | --------- | ----- | ------------------------------------------------- | ------------------ |
| `url`       | `string`  | ✅    | The rtmp:// or rtmps:// url of the target server. | Format: `uri`      |
| `active`    | `boolean` | ✅    | Whether this target is currently active.          |                    |
| `createdAt` | `string`  | ✅    | When this target was created.                     | Format: `datetime` |
| `name`      | `string`  | ❌    | A user-friendly name for this target.             | Max Length: 100    |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.multistream.target",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "An external server for rebroadcasting a Streamplace stream",
      "record": {
        "required": ["url", "active", "createdAt"],
        "type": "object",
        "properties": {
          "url": {
            "type": "string",
            "format": "uri",
            "description": "The rtmp:// or rtmps:// url of the target server."
          },
          "active": {
            "type": "boolean",
            "description": "Whether this target is currently active."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "When this target was created."
          },
          "name": {
            "type": "string",
            "maxLength": 100,
            "description": "A user-friendly name for this target."
          }
        }
      }
    }
  }
}
```
