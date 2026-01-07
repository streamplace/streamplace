---
title: place.stream.server.getServerTime
description: Reference for the place.stream.server.getServerTime lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get the current server time for client clock synchronization

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type     | Req'd | Description                           | Constraints        |
| ------------ | -------- | ----- | ------------------------------------- | ------------------ |
| `serverTime` | `string` | ✅    | Current server time in RFC3339 format | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.getServerTime",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get the current server time for client clock synchronization",
      "parameters": {
        "type": "params",
        "properties": {}
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["serverTime"],
          "properties": {
            "serverTime": {
              "type": "string",
              "format": "datetime",
              "description": "Current server time in RFC3339 format"
            }
          }
        }
      }
    }
  }
}
```
