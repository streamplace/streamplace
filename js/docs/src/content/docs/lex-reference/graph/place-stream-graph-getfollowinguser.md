---
title: place.stream.graph.getFollowingUser
description: Reference for the place.stream.graph.getFollowingUser lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get whether or not user A is following user B.

**Parameters:**

| Name         | Type     | Req'd | Description                                    | Constraints   |
| ------------ | -------- | ----- | ---------------------------------------------- | ------------- |
| `userDID`    | `string` | ✅    | The DID of the potentially-following user      | Format: `did` |
| `subjectDID` | `string` | ✅    | The DID of the user potentially being followed | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                                                                   | Req'd | Description | Constraints |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `follow` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.graph.getFollowingUser",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get whether or not user A is following user B.",
      "parameters": {
        "type": "params",
        "required": ["userDID", "subjectDID"],
        "properties": {
          "userDID": {
            "type": "string",
            "format": "did",
            "description": "The DID of the potentially-following user"
          },
          "subjectDID": {
            "type": "string",
            "format": "did",
            "description": "The DID of the user potentially being followed"
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": [],
          "properties": {
            "follow": {
              "type": "ref",
              "ref": "com.atproto.repo.strongRef"
            }
          }
        }
      }
    }
  }
}
```
