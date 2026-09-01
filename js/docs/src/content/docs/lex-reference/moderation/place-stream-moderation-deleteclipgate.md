---
title: place.stream.moderation.deleteClipGate
description: Reference for the place.stream.moderation.deleteClipGate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete a clip gate on behalf of a streamer. Requires 'clip.hide' permission. Deletes the place.stream.clip.gate record from the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                              | Constraints      |
| ---------- | -------- | ----- | ---------------------------------------- | ---------------- |
| `streamer` | `string` | ✅    | The DID of the streamer.                 | Format: `did`    |
| `uri`      | `string` | ✅    | The AT-URI of the gate record to delete. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_
**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.deleteClipGate",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete a clip gate on behalf of a streamer. Requires 'clip.hide' permission. Deletes the place.stream.clip.gate record from the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "uri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "uri": {
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
          "required": [],
          "properties": {}
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
