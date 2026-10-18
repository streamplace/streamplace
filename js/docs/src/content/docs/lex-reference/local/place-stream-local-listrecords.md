---
title: place.stream.local.listRecords
description: Reference for the place.stream.local.listRecords lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

List the caller's records of a collection kept on this node, in record-key order. Shaped like com.atproto.repo.listRecords.

**Parameters:**

| Name         | Type     | Req'd | Description | Constraints    |
| ------------ | -------- | ----- | ----------- | -------------- |
| `repo`       | `string` | ✅    |             | Format: `did`  |
| `collection` | `string` | ✅    |             | Format: `nsid` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                          | Req'd | Description | Constraints |
| --------- | ----------------------------- | ----- | ----------- | ----------- |
| `records` | Array of [`#record`](#record) | ✅    |             |             |

---

<a name="record"></a>

### `record`

**Type:** `object`

**Properties:**

| Name    | Type      | Req'd | Description | Constraints      |
| ------- | --------- | ----- | ----------- | ---------------- |
| `uri`   | `string`  | ✅    |             | Format: `at-uri` |
| `cid`   | `string`  | ✅    |             | Format: `cid`    |
| `value` | `unknown` | ✅    |             |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.local.listRecords",
  "defs": {
    "main": {
      "type": "query",
      "description": "List the caller's records of a collection kept on this node, in record-key order. Shaped like com.atproto.repo.listRecords.",
      "parameters": {
        "type": "params",
        "required": ["repo", "collection"],
        "properties": {
          "repo": {
            "type": "string",
            "format": "did"
          },
          "collection": {
            "type": "string",
            "format": "nsid"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["records"],
          "properties": {
            "records": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#record"
              }
            }
          }
        }
      }
    },
    "record": {
      "type": "object",
      "required": ["uri", "cid", "value"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "value": {
          "type": "unknown"
        }
      }
    }
  }
}
```
