---
title: place.stream.moderation.createBlock
description: Reference for the place.stream.moderation.createBlock lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create a block (ban) on behalf of a streamer. Requires 'ban' permission. Creates an app.bsky.graph.block record in the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                                               | Constraints     |
| ---------- | -------- | ----- | --------------------------------------------------------- | --------------- |
| `streamer` | `string` | ✅    | The DID of the streamer whose chat this block applies to. | Format: `did`   |
| `subject`  | `string` | ✅    | The DID of the user being blocked from chat.              | Format: `did`   |
| `reason`   | `string` | ❌    | Optional reason for the block.                            | Max Length: 300 |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                             | Constraints      |
| ----- | -------- | ----- | --------------------------------------- | ---------------- |
| `uri` | `string` | ✅    | The AT-URI of the created block record. | Format: `at-uri` |
| `cid` | `string` | ✅    | The CID of the created block record.    | Format: `cid`    |

**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `Forbidden`: The caller does not have permission to create blocks for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.createBlock",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create a block (ban) on behalf of a streamer. Requires 'ban' permission. Creates an app.bsky.graph.block record in the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "subject"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer whose chat this block applies to."
            },
            "subject": {
              "type": "string",
              "format": "did",
              "description": "The DID of the user being blocked from chat."
            },
            "reason": {
              "type": "string",
              "maxLength": 300,
              "description": "Optional reason for the block."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri", "cid"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the created block record."
            },
            "cid": {
              "type": "string",
              "format": "cid",
              "description": "The CID of the created block record."
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The request lacks valid authentication credentials."
        },
        {
          "name": "Forbidden",
          "description": "The caller does not have permission to create blocks for this streamer."
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
