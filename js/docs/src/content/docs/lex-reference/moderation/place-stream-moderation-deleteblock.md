---
title: place.stream.moderation.deleteBlock
description: Reference for the place.stream.moderation.deleteBlock lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete a block (unban) on behalf of a streamer. Requires 'ban' permission. Deletes an app.bsky.graph.block record from the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                               | Constraints      |
| ---------- | -------- | ----- | ----------------------------------------- | ---------------- |
| `streamer` | `string` | ✅    | The DID of the streamer.                  | Format: `did`    |
| `blockUri` | `string` | ✅    | The AT-URI of the block record to delete. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_
**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `Forbidden`: The caller does not have permission to delete blocks for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.deleteBlock",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete a block (unban) on behalf of a streamer. Requires 'ban' permission. Deletes an app.bsky.graph.block record from the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "blockUri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "blockUri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the block record to delete."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "properties": {}
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The request lacks valid authentication credentials."
        },
        {
          "name": "Forbidden",
          "description": "The caller does not have permission to delete blocks for this streamer."
        },
        {
          "name": "SessionNotFound",
          "description": "The streamer's OAuth session could not be found or is invalid."
        }
      ]
    }
  }
}
```
