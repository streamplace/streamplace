---
title: place.stream.media.updateVideo
description: Reference for the place.stream.media.updateVideo lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Rewrite a place.stream.video record in place: retitle, redescribe or retag it, or point it at a finished upload's tracks (publishing them if the upload has none yet: the repair for a record published without tracks, or the way to point a record at a re-finalized VOD). Fields left out are kept; an empty description or tag list clears it. The caller must be the record's owner or hold livestream.manage from them; the record is written with the owner's session.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type              | Req'd | Description                                                                                | Constraints       |
| ------------- | ----------------- | ----- | ------------------------------------------------------------------------------------------ | ----------------- |
| `uri`         | `string`          | ✅    | The place.stream.video record.                                                             | Format: `at-uri`  |
| `title`       | `string`          | ❌    |                                                                                            | Max Length: 1000  |
| `description` | `string`          | ❌    |                                                                                            | Max Length: 10000 |
| `tags`        | Array of `string` | ❌    |                                                                                            |                   |
| `uploadId`    | `string`          | ❌    | A finished upload whose tracks become the record's source and whose duration the record's. |                   |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description | Constraints      |
| ----- | -------- | ----- | ----------- | ---------------- |
| `uri` | `string` | ✅    |             | Format: `at-uri` |
| `cid` | `string` | ✅    |             | Format: `cid`    |

**Possible Errors:**

- `NotPermitted`: The caller is neither the owner nor a moderator with livestream.manage from them.
- `VideoNotFound`: No such video record.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.updateVideo",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Rewrite a place.stream.video record in place: retitle, redescribe or retag it, or point it at a finished upload's tracks (publishing them if the upload has none yet: the repair for a record published without tracks, or the way to point a record at a re-finalized VOD). Fields left out are kept; an empty description or tag list clears it. The caller must be the record's owner or hold livestream.manage from them; the record is written with the owner's session.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri",
              "description": "The place.stream.video record."
            },
            "title": {
              "type": "string",
              "maxLength": 1000
            },
            "description": {
              "type": "string",
              "maxLength": 10000
            },
            "tags": {
              "type": "array",
              "items": {
                "type": "string",
                "maxLength": 64
              }
            },
            "uploadId": {
              "type": "string",
              "description": "A finished upload whose tracks become the record's source and whose duration the record's."
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
              "format": "at-uri"
            },
            "cid": {
              "type": "string",
              "format": "cid"
            }
          }
        }
      },
      "errors": [
        {
          "name": "NotPermitted",
          "description": "The caller is neither the owner nor a moderator with livestream.manage from them."
        },
        {
          "name": "VideoNotFound",
          "description": "No such video record."
        }
      ]
    }
  }
}
```
