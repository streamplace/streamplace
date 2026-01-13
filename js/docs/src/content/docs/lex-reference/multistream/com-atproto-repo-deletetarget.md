---
title: com.atproto.repo.deleteTarget
description: Reference for the com.atproto.repo.deleteTarget lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete a target for rebroadcasting a Streamplace stream.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name   | Type     | Req'd | Description                             | Constraints          |
| ------ | -------- | ----- | --------------------------------------- | -------------------- |
| `rkey` | `string` | ✅    | The Record Key of the target to delete. | Format: `record-key` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "com.atproto.repo.deleteTarget",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete a target for rebroadcasting a Streamplace stream.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["rkey"],
          "properties": {
            "rkey": {
              "type": "string",
              "format": "record-key",
              "description": "The Record Key of the target to delete."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "properties": {}
        }
      },
      "errors": []
    }
  }
}
```
