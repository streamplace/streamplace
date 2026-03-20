---
title: place.stream.moderation.createPin
description: Reference for the place.stream.moderation.createPin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Pin a chat message on behalf of a streamer. Requires 'message.pin' permission. Creates a place.stream.chat.pinnedRecord in the streamer's repo, replacing any existing pin.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type     | Req'd | Description                            | Constraints        |
| ------------ | -------- | ----- | -------------------------------------- | ------------------ |
| `streamer`   | `string` | ✅    | The DID of the streamer.               | Format: `did`      |
| `messageUri` | `string` | ✅    | The AT-URI of the chat message to pin. | Format: `at-uri`   |
| `expiresAt`  | `string` | ❌    | Optional expiration time for this pin. | Format: `datetime` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                              | Constraints      |
| ----- | -------- | ----- | ---------------------------------------- | ---------------- |
| `uri` | `string` | ✅    | The AT-URI of the created pinned record. | Format: `at-uri` |
| `cid` | `string` | ✅    | The CID of the created pinned record.    | Format: `cid`    |

**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `Forbidden`: The caller does not have permission to pin messages for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.createPin",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Pin a chat message on behalf of a streamer. Requires 'message.pin' permission. Creates a place.stream.chat.pinnedRecord in the streamer's repo, replacing any existing pin.",
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
              "description": "The AT-URI of the chat message to pin."
            },
            "expiresAt": {
              "type": "string",
              "format": "datetime",
              "description": "Optional expiration time for this pin."
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
              "description": "The AT-URI of the created pinned record."
            },
            "cid": {
              "type": "string",
              "format": "cid",
              "description": "The CID of the created pinned record."
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
          "description": "The caller does not have permission to pin messages for this streamer."
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
