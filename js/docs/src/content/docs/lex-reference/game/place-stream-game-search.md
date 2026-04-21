---
title: place.stream.game.search
description: Reference for the place.stream.game.search lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Search for games and other entities via the games.gamesgamesgamesgames catalog. Proxied from the configured games API.

**Parameters:**

| Name     | Type      | Req'd | Description | Constraints                           |
| -------- | --------- | ----- | ----------- | ------------------------------------- |
| `q`      | `string`  | ✅    |             | Min Length: 1<br/>Max Length: 200     |
| `limit`  | `integer` | ❌    |             | Min: 1<br/>Max: 100<br/>Default: `20` |
| `cursor` | `string`  | ❌    |             |                                       |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name           | Type                                                                                                                                                                                                                                                                                                                                                                        | Req'd | Description | Constraints |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `cursor`       | `string`                                                                                                                                                                                                                                                                                                                                                                    | ❌    |             |             |
| `totalResults` | `integer`                                                                                                                                                                                                                                                                                                                                                                   | ❌    |             |             |
| `results`      | Array of Union of:<br/>&nbsp;&nbsp;`games.gamesgamesgamesgames.defs#gameSummaryView`<br/>&nbsp;&nbsp;`games.gamesgamesgamesgames.defs#profileSummaryView`<br/>&nbsp;&nbsp;`games.gamesgamesgamesgames.defs#platformSummaryView`<br/>&nbsp;&nbsp;`games.gamesgamesgamesgames.defs#collectionSummaryView`<br/>&nbsp;&nbsp;`games.gamesgamesgamesgames.defs#engineSummaryView` | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.game.search",
  "defs": {
    "main": {
      "type": "query",
      "description": "Search for games and other entities via the games.gamesgamesgamesgames catalog. Proxied from the configured games API.",
      "parameters": {
        "type": "params",
        "required": ["q"],
        "properties": {
          "q": {
            "type": "string",
            "minLength": 1,
            "maxLength": 200
          },
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 20
          },
          "cursor": {
            "type": "string"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["results"],
          "properties": {
            "cursor": {
              "type": "string"
            },
            "totalResults": {
              "type": "integer"
            },
            "results": {
              "type": "array",
              "items": {
                "type": "union",
                "refs": [
                  "games.gamesgamesgamesgames.defs#gameSummaryView",
                  "games.gamesgamesgamesgames.defs#profileSummaryView",
                  "games.gamesgamesgamesgames.defs#platformSummaryView",
                  "games.gamesgamesgamesgames.defs#collectionSummaryView",
                  "games.gamesgamesgamesgames.defs#engineSummaryView"
                ]
              }
            }
          }
        }
      }
    }
  }
}
```
