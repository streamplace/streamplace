---
title: place.stream.broadcast.syndication
description: Reference for the place.stream.broadcast.syndication lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record created by a Streamplace broadcaster to indicate that they will be
replicating a livestream. NYI

**Record Key:** `tid`

**Record Properties:**

| Name          | Type     | Req'd | Description                                                                | Constraints        |
| ------------- | -------- | ----- | -------------------------------------------------------------------------- | ------------------ |
| `broadcaster` | `string` | ✅    | DID of the Streamplace broadcaster that will be replicating the livestream | Format: `did`      |
| `streamer`    | `string` | ✅    | DID of the streamer whose livestream is being replicated                   | Format: `did`      |
| `createdAt`   | `string` | ✅    | Client-declared timestamp when this syndication was created.               | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.broadcast.syndication",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record created by a Streamplace broadcaster to indicate that they will be replicating a livestream. NYI",
      "record": {
        "type": "object",
        "required": ["broadcaster", "streamer", "createdAt"],
        "properties": {
          "broadcaster": {
            "type": "string",
            "format": "did",
            "description": "DID of the Streamplace broadcaster that will be replicating the livestream"
          },
          "streamer": {
            "type": "string",
            "format": "did",
            "description": "DID of the streamer whose livestream is being replicated"
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this syndication was created."
          }
        }
      }
    }
  }
}
```
