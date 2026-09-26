---
title: place.stream.broadcast.getBroadcaster
description: Reference for the place.stream.broadcast.getBroadcaster lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get information about a Streamplace broadcaster, as seen from the requested hostname.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type              | Req'd | Description                                                                                                                                   | Constraints   |
| ------------- | ----------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| `broadcaster` | `string`          | ✅    | DID of the Streamplace broadcaster to which this server belongs                                                                               | Format: `did` |
| `server`      | `string`          | ❌    | DID of this particular Streamplace server                                                                                                     | Format: `did` |
| `admins`      | Array of `string` | ❌    | Array of DIDs authorized as admins                                                                                                            |               |
| `brand`       | `string`          | ❌    | Broadcaster ID the requested hostname's branding is read and written under: did:web:<custom domain> on a custom domain, else the broadcaster. | Format: `did` |
| `brandAdmins` | Array of `string` | ❌    | DIDs that may change that branding: the admins, or on a custom domain its owner.                                                              |               |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.broadcast.getBroadcaster",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get information about a Streamplace broadcaster, as seen from the requested hostname.",
      "parameters": {
        "type": "params",
        "required": [],
        "properties": {}
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["broadcaster"],
          "properties": {
            "broadcaster": {
              "type": "string",
              "format": "did",
              "description": "DID of the Streamplace broadcaster to which this server belongs"
            },
            "server": {
              "type": "string",
              "format": "did",
              "description": "DID of this particular Streamplace server"
            },
            "admins": {
              "type": "array",
              "items": {
                "type": "string",
                "format": "did"
              },
              "description": "Array of DIDs authorized as admins"
            },
            "brand": {
              "type": "string",
              "format": "did",
              "description": "Broadcaster ID the requested hostname's branding is read and written under: did:web:<custom domain> on a custom domain, else the broadcaster."
            },
            "brandAdmins": {
              "type": "array",
              "items": {
                "type": "string",
                "format": "did"
              },
              "description": "DIDs that may change that branding: the admins, or on a custom domain its owner."
            }
          }
        }
      },
      "errors": []
    }
  }
}
```
