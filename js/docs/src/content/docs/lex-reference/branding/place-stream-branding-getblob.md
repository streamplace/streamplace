---
title: place.stream.branding.getBlob
description: Reference for the place.stream.branding.getBlob lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get a specific branding asset blob by key.

**Parameters:**

| Name          | Type     | Req'd | Description                                                                     | Constraints   |
| ------------- | -------- | ----- | ------------------------------------------------------------------------------- | ------------- |
| `key`         | `string` | ✅    | Branding asset key (mainLogo, favicon, siteTitle, etc.)                         |               |
| `broadcaster` | `string` | ❌    | DID of the broadcaster. If not provided, uses the server's default broadcaster. | Format: `did` |

**Output:**

- **Encoding:** `*/*`
- **Description:** Raw blob data with appropriate content-type
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `BrandingNotFound`: The requested branding asset does not exist

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.getBlob",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get a specific branding asset blob by key.",
      "parameters": {
        "type": "params",
        "required": ["key"],
        "properties": {
          "key": {
            "type": "string",
            "description": "Branding asset key (mainLogo, favicon, siteTitle, etc.)"
          },
          "broadcaster": {
            "type": "string",
            "format": "did",
            "description": "DID of the broadcaster. If not provided, uses the server's default broadcaster."
          }
        }
      },
      "output": {
        "encoding": "*/*",
        "description": "Raw blob data with appropriate content-type"
      },
      "errors": [
        {
          "name": "BrandingNotFound",
          "description": "The requested branding asset does not exist"
        }
      ]
    }
  }
}
```
