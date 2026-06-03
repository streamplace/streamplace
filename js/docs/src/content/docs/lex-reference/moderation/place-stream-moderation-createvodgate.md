---
title: place.stream.moderation.createVodGate
description: Reference for the place.stream.moderation.createVodGate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create a gate (hide VOD comment) on behalf of a streamer. Requires 'vod.comment.hide' permission. Creates a place.stream.vod.gate record in the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type     | Req'd | Description                            | Constraints      |
| ------------ | -------- | ----- | -------------------------------------- | ---------------- |
| `streamer`   | `string` | ✅    | The DID of the streamer.               | Format: `did`    |
| `commentUri` | `string` | ✅    | The AT-URI of the VOD comment to hide. | Format: `at-uri` |

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
- `Forbidden`: The caller does not have permission to hide VOD comments for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.createVodGate",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create a gate (hide VOD comment) on behalf of a streamer. Requires 'vod.comment.hide' permission. Creates a place.stream.vod.gate record in the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "commentUri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "commentUri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the VOD comment to hide."
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
          "description": "The caller does not have permission to hide VOD comments for this streamer."
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
