---
title: place.stream.caption.pushCaptions
description: Reference for the place.stream.caption.pushCaptions lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Push live captions into a stream from a captioner, a CART stenographer, or an external tool. Requires the streamer's OAuth authorization or active registered stream key. Under canonical policy auto or ingest, pushed captions become streamer-signed MUXL text tracks. Under canonical policy off, pushed captions remain sidecar or local according to the streamer's node-caption policy.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                                                                                                 | Req'd | Description                                                                                                                                                                                                                                            | Constraints                   |
| ---------- | ---------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------- |
| `streamer` | `string`                                                                                             | ❌    | Defaults to the authenticated account.                                                                                                                                                                                                                 | Format: `did`                 |
| `language` | `string`                                                                                             | ✅    |                                                                                                                                                                                                                                                        | Format: `language`            |
| `source`   | `string`                                                                                             | ❌    | Whether a person or a recognizer produced the captions. Defaults to human.                                                                                                                                                                             | Known Values: `human`, `auto` |
| `cues`     | Array of [`place.stream.caption.defs#pushedCue`](/lex-reference/place-stream-caption-defs#pushedcue) | ✅    | Each cue's startTime must be within 30 seconds before or after the server's request time. Its endTime must be later than startTime and no more than 30 seconds after it. If any cue violates these bounds, the entire batch is rejected with HTTP 400. | Max Items: 100                |

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
      "description": "Push live captions into a stream from a captioner, a CART stenographer, or an external tool. Requires the streamer's OAuth authorization or active registered stream key. Under canonical policy auto or ingest, pushed captions become streamer-signed MUXL text tracks. Under canonical policy off, pushed captions remain sidecar or local according to the streamer's node-caption policy.",
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
              "description": "Each cue's startTime must be within 30 seconds before or after the server's request time. Its endTime must be later than startTime and no more than 30 seconds after it. If any cue violates these bounds, the entire batch is rejected with HTTP 400.",
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
