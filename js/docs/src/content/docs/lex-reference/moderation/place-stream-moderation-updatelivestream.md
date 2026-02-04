---
title: place.stream.moderation.updateLivestream
description: Reference for the place.stream.moderation.updateLivestream lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Update livestream metadata on behalf of a streamer. Requires 'livestream.manage' permission. Updates a place.stream.livestream record in the streamer's repository.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name            | Type     | Req'd | Description                                    | Constraints                             |
| --------------- | -------- | ----- | ---------------------------------------------- | --------------------------------------- |
| `streamer`      | `string` | ✅    | The DID of the streamer.                       | Format: `did`                           |
| `livestreamUri` | `string` | ✅    | The AT-URI of the livestream record to update. | Format: `at-uri`                        |
| `title`         | `string` | ❌    | New title for the livestream.                  | Max Length: 1400<br/>Max Graphemes: 140 |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                                  | Constraints      |
| ----- | -------- | ----- | -------------------------------------------- | ---------------- |
| `uri` | `string` | ✅    | The AT-URI of the updated livestream record. | Format: `at-uri` |
| `cid` | `string` | ✅    | The CID of the updated livestream record.    | Format: `cid`    |

**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `Forbidden`: The caller does not have permission to update livestream metadata for this streamer.
- `SessionNotFound`: The streamer's OAuth session could not be found or is invalid.
- `RecordNotFound`: The specified livestream record does not exist.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.updateLivestream",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Update livestream metadata on behalf of a streamer. Requires 'livestream.manage' permission. Updates a place.stream.livestream record in the streamer's repository.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "livestreamUri"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "livestreamUri": {
              "type": "string",
              "format": "at-uri",
              "description": "The AT-URI of the livestream record to update."
            },
            "title": {
              "type": "string",
              "maxLength": 1400,
              "maxGraphemes": 140,
              "description": "New title for the livestream."
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
              "description": "The AT-URI of the updated livestream record."
            },
            "cid": {
              "type": "string",
              "format": "cid",
              "description": "The CID of the updated livestream record."
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
          "description": "The caller does not have permission to update livestream metadata for this streamer."
        },
        {
          "name": "SessionNotFound",
          "description": "The streamer's OAuth session could not be found or is invalid."
        },
        {
          "name": "RecordNotFound",
          "description": "The specified livestream record does not exist."
        }
      ]
    }
  }
}
```
