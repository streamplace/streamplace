---
title: place.stream.local.deleteRecord
description: Reference for the place.stream.local.deleteRecord lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete a record the caller keeps on this node. Shaped like com.atproto.repo.deleteRecord; deleting one that doesn't exist succeeds.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type     | Req'd | Description | Constraints                              |
| ------------ | -------- | ----- | ----------- | ---------------------------------------- |
| `repo`       | `string` | ✅    |             | Format: `did`                            |
| `collection` | `string` | ✅    |             | Format: `nsid`                           |
| `rkey`       | `string` | ✅    |             | Format: `record-key`<br/>Max Length: 512 |

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
  "id": "place.stream.local.deleteRecord",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete a record the caller keeps on this node. Shaped like com.atproto.repo.deleteRecord; deleting one that doesn't exist succeeds.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["repo", "collection", "rkey"],
          "properties": {
            "repo": {
              "type": "string",
              "format": "did"
            },
            "collection": {
              "type": "string",
              "format": "nsid"
            },
            "rkey": {
              "type": "string",
              "format": "record-key",
              "maxLength": 512
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
      }
    }
  }
}
```
