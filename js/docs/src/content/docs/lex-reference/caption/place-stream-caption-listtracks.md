---
title: place.stream.caption.listTracks
description: Reference for the place.stream.caption.listTracks lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

List the caption tracks available for a live stream on this node, or for a video.

**Parameters:**

| Name       | Type     | Req'd | Description                                                      | Constraints      |
| ---------- | -------- | ----- | ---------------------------------------------------------------- | ---------------- |
| `streamer` | `string` | ❌    | DID or handle of a live streamer. Mutually exclusive with video. |                  |
| `video`    | `string` | ❌    | AT-URI of a place.stream.video.                                  | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                                 | Req'd | Description | Constraints |
| -------- | ---------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `tracks` | Array of [`place.stream.caption.defs#trackView`](/lex-reference/place-stream-caption-defs#trackview) | ✅    |             |             |

**Possible Errors:**

- `NotFound`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.caption.listTracks",
  "defs": {
    "main": {
      "type": "query",
      "description": "List the caption tracks available for a live stream on this node, or for a video.",
      "parameters": {
        "type": "params",
        "properties": {
          "streamer": {
            "type": "string",
            "description": "DID or handle of a live streamer. Mutually exclusive with video."
          },
          "video": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of a place.stream.video."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["tracks"],
          "properties": {
            "tracks": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.caption.defs#trackView"
              }
            }
          }
        }
      },
      "errors": [
        {
          "name": "NotFound"
        }
      ]
    }
  }
}
```
