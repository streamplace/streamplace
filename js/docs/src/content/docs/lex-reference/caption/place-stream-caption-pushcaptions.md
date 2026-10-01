---
title: place.stream.caption.pushCaptions
description: Reference for the place.stream.caption.pushCaptions lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Push live captions into a stream from a captioner: the Streamplace caption overlay, a CART stenographer, or an external tool. Requires the streamer's authorization. When the streamer's caption policy is `ingest`, pushed captions become the canonical caption track.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                                                                                                 | Req'd | Description                                                                | Constraints                   |
| ---------- | ---------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------------- | ----------------------------- |
| `streamer` | `string`                                                                                             | ❌    | Defaults to the authenticated account.                                     | Format: `did`                 |
| `language` | `string`                                                                                             | ✅    |                                                                            | Format: `language`            |
| `source`   | `string`                                                                                             | ❌    | Whether a person or a recognizer produced the captions. Defaults to human. | Known Values: `human`, `auto` |
| `cues`     | Array of [`place.stream.caption.defs#pushedCue`](/lex-reference/place-stream-caption-defs#pushedcue) | ✅    |                                                                            | Max Items: 100                |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

_(No properties defined)_
**Possible Errors:**

- `StreamNotLive`
- `Forbidden`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.caption.pushCaptions",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Push live captions into a stream from a captioner: the Streamplace caption overlay, a CART stenographer, or an external tool. Requires the streamer's authorization. When the streamer's caption policy is `ingest`, pushed captions become the canonical caption track.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["language", "cues"],
          "properties": {
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "Defaults to the authenticated account."
            },
            "language": {
              "type": "string",
              "format": "language"
            },
            "source": {
              "type": "string",
              "knownValues": ["human", "auto"],
              "description": "Whether a person or a recognizer produced the captions. Defaults to human."
            },
            "cues": {
              "type": "array",
              "maxLength": 100,
              "items": {
                "type": "ref",
                "ref": "place.stream.caption.defs#pushedCue"
              }
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "properties": {}
        }
      },
      "errors": [
        {
          "name": "StreamNotLive"
        },
        {
          "name": "Forbidden"
        }
      ]
    }
  }
}
```
