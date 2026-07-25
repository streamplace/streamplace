---
title: place.stream.moderation.deletePin
description: Reference for the place.stream.moderation.deletePin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Unpin a pinned chat message on behalf of a streamer. Requires 'message.pin' permission. Deletes the place.stream.chat.pinnedRecord from the streamer's repo.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                                | Constraints      |
| ---------- | -------- | ----- | ------------------------------------------ | ---------------- |
| `streamer` | `string` | ✅    | The DID of the streamer.                   | Format: `did`    |
| `pinUri`   | `string` | ✅    | The AT-URI of the pinned record to delete. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_
**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `Forbidden`: The caller does not have permission to unpin messages for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.deletePin",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Unpin a pinned chat message on behalf of a streamer. Requires 'message.pin' permission. Deletes the place.stream.chat.pinnedRecord from the streamer's repo.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "pinUri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "pinUri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the pinned record to delete."
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
          "description": "The caller does not have permission to unpin messages for this streamer."
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
