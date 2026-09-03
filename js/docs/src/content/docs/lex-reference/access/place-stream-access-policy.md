---
title: place.stream.access.policy
description: Reference for the place.stream.access.policy lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

The node's access policy: the mode of each role. A single record authored by the space authority in the node's access-control space. Roles absent from the record use the node's defaults.

**Record Key:** `literal:self`

**Record Properties:**

| Name        | Type                                                                                             | Req'd | Description | Constraints        |
| ----------- | ------------------------------------------------------------------------------------------------ | ----- | ----------- | ------------------ |
| `roles`     | Array of [`place.stream.access.defs#roleMode`](/lex-reference/place-stream-access-defs#rolemode) | ✅    |             |                    |
| `updatedAt` | `string`                                                                                         | ✅    |             | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.policy",
  "defs": {
    "main": {
      "type": "record",
      "key": "literal:self",
      "description": "The node's access policy: the mode of each role. A single record authored by the space authority in the node's access-control space. Roles absent from the record use the node's defaults.",
      "record": {
        "type": "object",
        "required": ["roles", "updatedAt"],
        "properties": {
          "roles": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "place.stream.access.defs#roleMode"
            }
          },
          "updatedAt": {
            "type": "string",
            "format": "datetime"
          }
        }
      }
    }
  }
}
```
