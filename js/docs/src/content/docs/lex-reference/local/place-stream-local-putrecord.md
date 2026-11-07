---
title: place.stream.local.putRecord
description: Reference for the place.stream.local.putRecord lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Write a record that stays on this node instead of the caller's repo, such as its configuration for this node. Shaped like com.atproto.repo.putRecord.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type      | Req'd | Description                                                             | Constraints                              |
| ------------ | --------- | ----- | ----------------------------------------------------------------------- | ---------------------------------------- |
| `repo`       | `string`  | ✅    | The caller's DID; a node keeps local records for their own author only. | Format: `did`                            |
| `collection` | `string`  | ✅    | One of the collections this node keeps locally.                         | Format: `nsid`                           |
| `rkey`       | `string`  | ✅    |                                                                         | Format: `record-key`<br/>Max Length: 512 |
| `record`     | `unknown` | ✅    |                                                                         |                                          |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description | Constraints      |
| ----- | -------- | ----- | ----------- | ---------------- |
| `uri` | `string` | ✅    |             | Format: `at-uri` |
| `cid` | `string` | ✅    |             | Format: `cid`    |

**Possible Errors:**

- `InvalidCollection`: This node does not keep records of the collection locally.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.local.putRecord",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Write a record that stays on this node instead of the caller's repo, such as its configuration for this node. Shaped like com.atproto.repo.putRecord.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["repo", "collection", "rkey", "record"],
          "properties": {
            "repo": {
              "type": "string",
              "format": "did",
              "description": "The caller's DID; a node keeps local records for their own author only."
            },
            "collection": {
              "type": "string",
              "format": "nsid",
              "description": "One of the collections this node keeps locally."
            },
            "rkey": {
              "type": "string",
              "format": "record-key",
              "maxLength": 512
            },
            "record": {
              "type": "unknown"
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri", "cid"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri"
            },
            "cid": {
              "type": "string",
              "format": "cid"
            }
          }
        }
      },
      "errors": [
        {
          "name": "InvalidCollection",
          "description": "This node does not keep records of the collection locally."
        }
      ]
    }
  }
}
```
