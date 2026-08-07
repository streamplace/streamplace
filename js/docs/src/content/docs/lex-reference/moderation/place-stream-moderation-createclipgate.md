---
title: place.stream.moderation.createClipGate
description: Reference for the place.stream.moderation.createClipGate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create a gate (hide clip) on behalf of a streamer. Requires 'clip.hide' permission. Creates a place.stream.clip.gate record in the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                                               | Constraints      |
| ---------- | -------- | ----- | --------------------------------------------------------- | ---------------- |
| `streamer` | `string` | ✅    | The DID of the streamer.                                  | Format: `did`    |
| `clipUri`  | `string` | ✅    | The AT-URI of the place.stream.clip.entry record to hide. | Format: `at-uri` |
| `clipCid`  | `string` | ❌    | The CID of the place.stream.clip.entry record to hide.    | Format: `cid`    |

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

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.createClipGate",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create a gate (hide clip) on behalf of a streamer. Requires 'clip.hide' permission. Creates a place.stream.clip.gate record in the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "clipUri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "clipUri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the place.stream.clip.entry record to hide."
            },
            "clipCid": {
              "type": "string",
              "format": "cid",
              "description": "The CID of the place.stream.clip.entry record to hide."
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
        }
      ]
    }
  }
}
```
