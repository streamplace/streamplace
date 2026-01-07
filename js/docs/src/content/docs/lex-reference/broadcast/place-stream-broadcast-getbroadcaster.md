---
title: place.stream.broadcast.getBroadcaster
description: Reference for the place.stream.broadcast.getBroadcaster lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get information about a Streamplace broadcaster.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type     | Req'd | Description                                                     | Constraints   |
| ------------- | -------- | ----- | --------------------------------------------------------------- | ------------- |
| `broadcaster` | `string` | ✅    | DID of the Streamplace broadcaster to which this server belongs | Format: `did` |
| `server`      | `string` | ❌    | DID of this particular Streamplace server                       | Format: `did` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.broadcast.getBroadcaster",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get information about a Streamplace broadcaster.",
      "parameters": {
        "type": "params",
        "required": [],
        "properties": {}
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["broadcaster"],
          "properties": {
            "broadcaster": {
              "type": "string",
              "format": "did",
              "description": "DID of the Streamplace broadcaster to which this server belongs"
            },
            "server": {
              "type": "string",
              "format": "did",
              "description": "DID of this particular Streamplace server"
            }
          }
        }
      },
      "errors": []
    }
  }
}
```
