---
title: place.stream.game.getGame
description: Reference for the place.stream.game.getGame lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

**Parameters:**

| Name  | Type     | Req'd | Description | Constraints      |
| ----- | -------- | ----- | ----------- | ---------------- |
| `uri` | `string` | ✅    |             | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type              | Req'd | Description | Constraints      |
| ---------- | ----------------- | ----- | ----------- | ---------------- |
| `uri`      | `string`          | ✅    |             | Format: `at-uri` |
| `name`     | `string`          | ✅    |             |                  |
| `summary`  | `string`          | ❌    |             |                  |
| `coverUrl` | `string`          | ❌    |             |                  |
| `genres`   | Array of `string` | ❌    |             |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.game.getGame",
  "defs": {
    "main": {
      "type": "query",
      "parameters": {
        "type": "params",
        "required": ["uri"],
        "properties": {
          "uri": {
            "type": "string",
            "format": "at-uri"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri", "name"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri"
            },
            "name": {
              "type": "string"
            },
            "summary": {
              "type": "string"
            },
            "coverUrl": {
              "type": "string"
            },
            "genres": {
              "type": "array",
              "items": {
                "type": "string"
              }
            }
          }
        }
      }
    }
  }
}
```
