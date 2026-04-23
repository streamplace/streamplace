---
title: place.stream.moderation.createGate
description: Reference for the place.stream.moderation.createGate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create a gate (hide message) on behalf of a streamer. Requires 'hide' permission. Creates a place.stream.chat.gate record in the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type     | Req'd | Description                             | Constraints      |
| ------------ | -------- | ----- | --------------------------------------- | ---------------- |
| `streamer`   | `string` | ✅    | The DID of the streamer.                | Format: `did`    |
| `messageUri` | `string` | ✅    | The AT-URI of the chat message to hide. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                            | Constraints      |
| ----- | -------- | ----- | -------------------------------------- | ---------------- |
| `uri` | `string` | ✅    | The AT-URI of the created gate record. | Format: `at-uri` |
| `cid` | `string` | ✅    | The CID of the created gate record.    | Format: `cid`    |

**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `Forbidden`: The caller does not have permission to hide messages for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.createGate",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create a gate (hide message) on behalf of a streamer. Requires 'hide' permission. Creates a place.stream.chat.gate record in the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "messageUri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "messageUri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the chat message to hide."
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
              "description": "The AT-URI of the created gate record."
            },
            "cid": {
              "type": "string",
              "format": "cid",
              "description": "The CID of the created gate record."
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
          "description": "The caller does not have permission to hide messages for this streamer."
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
