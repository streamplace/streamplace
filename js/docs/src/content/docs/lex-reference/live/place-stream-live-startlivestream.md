---
title: place.stream.live.startLivestream
description: Reference for the place.stream.live.startLivestream lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create a new place.stream.livestream record, automatically populating a thumbnail and creating a Bluesky post and whatnot. You can do this manually by creating a record but this method can work better for mobile livestreaming and such.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name                | Type                                                                | Req'd | Description                                                 | Constraints   |
| ------------------- | ------------------------------------------------------------------- | ----- | ----------------------------------------------------------- | ------------- |
| `livestream`        | [`place.stream.livestream`](/lex-reference/place-stream-livestream) | ✅    |                                                             |               |
| `streamer`          | `string`                                                            | ✅    | The DID of the streamer.                                    | Format: `did` |
| `createBlueskyPost` | `boolean`                                                           | ❌    | Whether to create a Bluesky post announcing the livestream. |               |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                       | Constraints   |
| ----- | -------- | ----- | --------------------------------- | ------------- |
| `uri` | `string` | ✅    | The URI of the livestream record. | Format: `uri` |
| `cid` | `string` | ✅    | The CID of the livestream record. | Format: `cid` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.startLivestream",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create a new place.stream.livestream record, automatically populating a thumbnail and creating a Bluesky post and whatnot. You can do this manually by creating a record but this method can work better for mobile livestreaming and such.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "livestream"],
          "properties": {
            "livestream": {
              "type": "ref",
              "ref": "place.stream.livestream"
            },
            "streamer": {
              "type": "string",
              "format": "did",
              "description": "The DID of the streamer."
            },
            "createBlueskyPost": {
              "type": "boolean",
              "description": "Whether to create a Bluesky post announcing the livestream."
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
              "format": "uri",
              "description": "The URI of the livestream record."
            },
            "cid": {
              "type": "string",
              "format": "cid",
              "description": "The CID of the livestream record."
            }
          }
        }
      }
    }
  }
}
```
