---
title: place.stream.vod.publishDraft
description: Reference for the place.stream.vod.publishDraft lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Promote a ready draft VOD to a public place.stream.video record. The server carries over source and durationMs from the draft, backfills a generated thumbnail if the draft has none, writes the video record via putRecord, then deletes the draft.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                                                       | Constraints |
| ----- | -------- | ----- | ----------------------------------------------------------------- | ----------- |
| `uri` | `string` | ✅    | The ats:// URI of the draft to publish. Must have status 'ready'. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                                      | Constraints      |
| ---------- | -------- | ----- | ------------------------------------------------ | ---------------- |
| `videoUri` | `string` | ✅    | AT-URI of the created place.stream.video record. | Format: `at-uri` |
| `videoCid` | `string` | ✅    | CID of the created place.stream.video record.    | Format: `cid`    |

**Possible Errors:**

- `NotFound`: No draft exists with the given URI for the authenticated user.
- `NotReady`: The draft is not ready to publish (status is 'processing' or 'error').

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.publishDraft",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Promote a ready draft VOD to a public place.stream.video record. The server carries over source and durationMs from the draft, backfills a generated thumbnail if the draft has none, writes the video record via putRecord, then deletes the draft.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri"],
          "properties": {
            "uri": {
              "type": "string",
              "description": "The ats:// URI of the draft to publish. Must have status 'ready'."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["videoUri", "videoCid"],
          "properties": {
            "videoUri": {
              "type": "string",
              "format": "at-uri",
              "description": "AT-URI of the created place.stream.video record."
            },
            "videoCid": {
              "type": "string",
              "format": "cid",
              "description": "CID of the created place.stream.video record."
            }
          }
        }
      },
      "errors": [
        {
          "name": "NotFound",
          "description": "No draft exists with the given URI for the authenticated user."
        },
        {
          "name": "NotReady",
          "description": "The draft is not ready to publish (status is 'processing' or 'error')."
        }
      ]
    }
  }
}
```
