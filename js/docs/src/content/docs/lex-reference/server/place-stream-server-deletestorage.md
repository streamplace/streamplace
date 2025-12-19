---
title: place.stream.server.deleteStorage
description: Reference for the place.stream.server.deleteStorage lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete S3 storage configuration.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description | Constraints |
| --------- | --------- | ----- | ----------- | ----------- |
| `success` | `boolean` | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.deleteStorage",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete S3 storage configuration.",
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["success"],
          "properties": {
            "success": {
              "type": "boolean"
            }
          }
        }
      }
    }
  }
}
```
