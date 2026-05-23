---
title: place.stream.config.getEnv
description: Reference for the place.stream.config.getEnv lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get client-facing environment configuration from the server.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name                | Type      | Req'd | Description                                       | Constraints |
| ------------------- | --------- | ----- | ------------------------------------------------- | ----------- |
| `playbackWorkerUrl` | `string`  | ❌    | URL of the Cloudflare playback router worker      |             |
| `gamesEnabled`      | `boolean` | ❌    | Whether the games API is configured and available |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.config.getEnv",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get client-facing environment configuration from the server.",
      "parameters": {
        "type": "params",
        "required": [],
        "properties": {}
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": [],
          "properties": {
            "playbackWorkerUrl": {
              "type": "string",
              "description": "URL of the Cloudflare playback router worker"
            },
            "gamesEnabled": {
              "type": "boolean",
              "description": "Whether the games API is configured and available"
            }
          }
        }
      },
      "errors": []
    }
  }
}
```
