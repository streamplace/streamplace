---
title: place.stream.server.getPreferences
description: Reference for the place.stream.server.getPreferences lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get the authenticated user's private preferences on this node.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type                                                                                          | Req'd | Description | Constraints |
| ------------- | --------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `preferences` | [`place.stream.server.defs#preferences`](/lex-reference/place-stream-server-defs#preferences) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.getPreferences",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get the authenticated user's private preferences on this node.",
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["preferences"],
          "properties": {
            "preferences": {
              "type": "ref",
              "ref": "place.stream.server.defs#preferences"
            }
          }
        }
      }
    }
  }
}
```
