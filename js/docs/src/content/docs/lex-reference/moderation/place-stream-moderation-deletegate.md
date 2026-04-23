---
title: place.stream.moderation.deleteGate
description: Reference for the place.stream.moderation.deleteGate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete a gate (unhide message) on behalf of a streamer. Requires 'hide' permission. Deletes a place.stream.chat.gate record from the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                              | Constraints      |
| ---------- | -------- | ----- | ---------------------------------------- | ---------------- |
| `streamer` | `string` | ✅    | The DID of the streamer.                 | Format: `did`    |
| `gateUri`  | `string` | ✅    | The AT-URI of the gate record to delete. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_
**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `Forbidden`: The caller does not have permission to unhide messages for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.deleteGate",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete a gate (unhide message) on behalf of a streamer. Requires 'hide' permission. Deletes a place.stream.chat.gate record from the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "gateUri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "gateUri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the gate record to delete."
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
          "description": "The caller does not have permission to unhide messages for this streamer."
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
