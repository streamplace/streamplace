---
title: place.stream.live.getProfileCard
description: Reference for the place.stream.live.getProfileCard lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get an OG image associated with a given account.

**Parameters:**

| Name | Type     | Req'd | Description                       | Constraints |
| ---- | -------- | ----- | --------------------------------- | ----------- |
| `id` | `string` | ✅    | The DID or handle of the account. |             |

**Output:**

- **Encoding:** `*/*`
- **Schema:**

_Schema not defined._ **Possible Errors:**

- `RepoNotFound`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.getProfileCard",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get an OG image associated with a given account.",
      "parameters": {
        "type": "params",
        "required": ["id"],
        "properties": {
          "id": {
            "type": "string",
            "description": "The DID or handle of the account."
          }
        }
      },
      "output": {
        "encoding": "*/*"
      },
      "errors": [
        {
          "name": "RepoNotFound"
        }
      ]
    }
  }
}
```
