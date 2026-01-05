---
title: place.stream.live.getRecommendations
description: Reference for the place.stream.live.getRecommendations lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get the list of streamers recommended by a user

**Parameters:**

| Name      | Type     | Req'd | Description                                        | Constraints   |
| --------- | -------- | ----- | -------------------------------------------------- | ------------- |
| `userDID` | `string` | ✅    | The DID of the user whose recommendations to fetch | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name              | Type                                                                                        | Req'd | Description                             | Constraints   |
| ----------------- | ------------------------------------------------------------------------------------------- | ----- | --------------------------------------- | ------------- |
| `recommendations` | Array of Union of:<br/>&nbsp;&nbsp;[`#livestreamRecommendation`](#livestreamrecommendation) | ✅    | Ordered list of recommendations         |               |
| `userDID`         | `string`                                                                                    | ❌    | The user DID this recommendation is for | Format: `did` |

---

<a name="livestreamrecommendation"></a>

### `livestreamRecommendation`

**Type:** `object`

**Properties:**

| Name     | Type     | Req'd | Description                         | Constraints                         |
| -------- | -------- | ----- | ----------------------------------- | ----------------------------------- |
| `did`    | `string` | ✅    | The DID of the recommended streamer | Format: `did`                       |
| `source` | `string` | ✅    | Source of the recommendation        | Enum: `streamer`, `follows`, `host` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.getRecommendations",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get the list of streamers recommended by a user",
      "parameters": {
        "type": "params",
        "required": ["userDID"],
        "properties": {
          "userDID": {
            "type": "string",
            "format": "did",
            "description": "The DID of the user whose recommendations to fetch"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["recommendations"],
          "properties": {
            "recommendations": {
              "type": "array",
              "description": "Ordered list of recommendations",
              "items": {
                "type": "union",
                "refs": ["#livestreamRecommendation"]
              }
            },
            "userDID": {
              "type": "string",
              "format": "did",
              "description": "The user DID this recommendation is for"
            }
          }
        }
      }
    },
    "livestreamRecommendation": {
      "type": "object",
      "required": ["did", "source"],
      "properties": {
        "did": {
          "type": "string",
          "format": "did",
          "description": "The DID of the recommended streamer"
        },
        "source": {
          "type": "string",
          "enum": ["streamer", "follows", "host"],
          "description": "Source of the recommendation"
        }
      }
    }
  }
}
```
