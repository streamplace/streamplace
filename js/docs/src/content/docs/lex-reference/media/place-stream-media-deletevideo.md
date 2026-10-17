---
title: place.stream.media.deleteVideo
description: Reference for the place.stream.media.deleteVideo lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete a place.stream.video record and, on request, the track records its source points at. The content blob and the upload stay, so the record can be published again from the same upload. The caller must be the record's owner or hold livestream.manage from them; the records are deleted with the owner's session.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type      | Req'd | Description                            | Constraints      |
| -------- | --------- | ----- | -------------------------------------- | ---------------- |
| `uri`    | `string`  | ✅    |                                        | Format: `at-uri` |
| `tracks` | `boolean` | ❌    | Also delete the video's track records. | Default: `false` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type              | Req'd | Description                            | Constraints |
| --------- | ----------------- | ----- | -------------------------------------- | ----------- |
| `deleted` | Array of `string` | ✅    | Every record deleted, the video first. |             |

**Possible Errors:**

- `NotPermitted`: The caller is neither the owner nor a moderator with livestream.manage from them.
- `VideoNotFound`: No such video record.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.deleteVideo",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete a place.stream.video record and, on request, the track records its source points at. The content blob and the upload stay, so the record can be published again from the same upload. The caller must be the record's owner or hold livestream.manage from them; the records are deleted with the owner's session.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri"
            },
            "tracks": {
              "type": "boolean",
              "default": false,
              "description": "Also delete the video's track records."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["deleted"],
          "properties": {
            "deleted": {
              "type": "array",
              "items": {
                "type": "string",
                "format": "at-uri"
              },
              "description": "Every record deleted, the video first."
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
