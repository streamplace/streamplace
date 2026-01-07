---
title: place.stream.broadcast.origin
description: Reference for the place.stream.broadcast.origin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record indicating a livestream is published and available for replication at a
given address. By convention, the record key is streamer::server

**Record Key:** `any`

**Record Properties:**

| Name           | Type     | Req'd | Description                                                                | Constraints        |
| -------------- | -------- | ----- | -------------------------------------------------------------------------- | ------------------ |
| `streamer`     | `string` | ✅    | DID of the streamer whose livestream is being published                    | Format: `did`      |
| `server`       | `string` | ✅    | did of the server that's currently rebroadcasting the livestream           | Format: `did`      |
| `broadcaster`  | `string` | ❌    | did of the broadcaster that operates the server syndicating the livestream | Format: `did`      |
| `updatedAt`    | `string` | ✅    | Periodically updated timestamp when this origin last saw a livestream      | Format: `datetime` |
| `irohTicket`   | `string` | ❌    | Iroh ticket that can be used to access the livestream from the server      | Max Length: 2048   |
| `websocketURL` | `string` | ❌    | URL of the websocket endpoint for the livestream                           | Format: `uri`      |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.broadcast.origin",
  "defs": {
    "main": {
      "type": "record",
      "key": "any",
      "description": "Record indicating a livestream is published and available for replication at a given address. By convention, the record key is streamer::server",
      "record": {
        "type": "object",
        "required": ["streamer", "server", "updatedAt"],
        "properties": {
          "streamer": {
            "type": "string",
            "format": "did",
            "description": "DID of the streamer whose livestream is being published"
          },
          "server": {
            "type": "string",
            "format": "did",
            "description": "did of the server that's currently rebroadcasting the livestream"
          },
          "broadcaster": {
            "type": "string",
            "format": "did",
            "description": "did of the broadcaster that operates the server syndicating the livestream"
          },
          "updatedAt": {
            "type": "string",
            "format": "datetime",
            "description": "Periodically updated timestamp when this origin last saw a livestream"
          },
          "irohTicket": {
            "type": "string",
            "maxLength": 2048,
            "description": "Iroh ticket that can be used to access the livestream from the server"
          },
          "websocketURL": {
            "type": "string",
            "format": "uri",
            "description": "URL of the websocket endpoint for the livestream"
          }
        }
      }
    }
  }
}
```
