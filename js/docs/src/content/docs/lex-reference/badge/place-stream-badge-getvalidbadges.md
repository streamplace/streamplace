---
title: place.stream.badge.getValidBadges
description: Reference for the place.stream.badge.getValidBadges lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get valid badges for the authenticated user, optionally in the context of a specific streamer's chat

**Parameters:**

| Name       | Type     | Req'd | Description                                                              | Constraints   |
| ---------- | -------- | ----- | ------------------------------------------------------------------------ | ------------- |
| `streamer` | `string` | ❌    | Optional DID of the streamer for context-specific badges (mod, vip, etc) | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                             | Req'd | Description | Constraints |
| -------- | ------------------------------------------------------------------------------------------------ | ----- | ----------- | ----------- |
| `badges` | Array of [`place.stream.badge.defs#badgeView`](/lex-reference/place-stream-badge-defs#badgeview) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.getValidBadges",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get valid badges for the authenticated user, optionally in the context of a specific streamer's chat",
      "parameters": {
        "type": "params",
        "properties": {
          "streamer": {
            "type": "string",
            "format": "did",
            "description": "Optional DID of the streamer for context-specific badges (mod, vip, etc)"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["badges"],
          "properties": {
            "badges": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.badge.defs#badgeView"
              }
            }
          }
        }
      }
    }
  }
}
```
