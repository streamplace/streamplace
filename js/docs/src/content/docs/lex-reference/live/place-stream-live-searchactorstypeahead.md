---
title: place.stream.live.searchActorsTypeahead
description: Reference for the place.stream.live.searchActorsTypeahead lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Find actor suggestions for a prefix search term. Expected use is for
auto-completion during text field entry.

**Parameters:**

| Name    | Type      | Req'd | Description                                   | Constraints                           |
| ------- | --------- | ----- | --------------------------------------------- | ------------------------------------- |
| `q`     | `string`  | ❌    | Search query prefix; not a full query string. |                                       |
| `limit` | `integer` | ❌    |                                               | Min: 1<br/>Max: 100<br/>Default: `10` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                        | Req'd | Description | Constraints |
| -------- | --------------------------- | ----- | ----------- | ----------- |
| `actors` | Array of [`#actor`](#actor) | ✅    |             |             |

---

<a name="actor"></a>

### `actor`

**Type:** `object`

**Properties:**

| Name     | Type     | Req'd | Description        | Constraints      |
| -------- | -------- | ----- | ------------------ | ---------------- |
| `did`    | `string` | ✅    | The actor's DID    | Format: `did`    |
| `handle` | `string` | ✅    | The actor's handle | Format: `handle` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.searchActorsTypeahead",
  "defs": {
    "main": {
      "type": "query",
      "description": "Find actor suggestions for a prefix search term. Expected use is for auto-completion during text field entry.",
      "parameters": {
        "type": "params",
        "properties": {
          "q": {
            "type": "string",
            "description": "Search query prefix; not a full query string."
          },
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 10
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["actors"],
          "properties": {
            "actors": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#actor"
              }
            }
          }
        }
      }
    },
    "actor": {
      "type": "object",
      "required": ["did", "handle"],
      "properties": {
        "did": {
          "type": "string",
          "format": "did",
          "description": "The actor's DID"
        },
        "handle": {
          "type": "string",
          "format": "handle",
          "description": "The actor's handle"
        }
      }
    }
  }
}
```
