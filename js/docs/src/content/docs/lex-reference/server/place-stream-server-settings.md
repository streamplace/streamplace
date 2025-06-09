---
title: place.stream.server.settings
description: Reference for the place.stream.server.settings lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record containing user settings for a particular Streamplace node

**Record Key:** `any`

**Record Properties:**

| Name             | Type      | Req'd | Description                                                             | Constraints |
| ---------------- | --------- | ----- | ----------------------------------------------------------------------- | ----------- |
| `debugRecording` | `boolean` | ❌    | Whether this node may archive your livestream for improving the service |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.settings",
  "defs": {
    "main": {
      "type": "record",
      "description": "Record containing user settings for a particular Streamplace node",
      "key": "any",
      "record": {
        "type": "object",
        "required": [],
        "properties": {
          "debugRecording": {
            "type": "boolean",
            "description": "Whether this node may archive your livestream for improving the service"
          }
        }
      }
    }
  }
}
```
