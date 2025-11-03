---
title: place.stream.branding.getBranding
description: Reference for the place.stream.branding.getBranding lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get all branding configuration for the broadcaster.

**Parameters:**

| Name          | Type     | Req'd | Description                                                                     | Constraints   |
| ------------- | -------- | ----- | ------------------------------------------------------------------------------- | ------------- |
| `broadcaster` | `string` | ❌    | DID of the broadcaster. If not provided, uses the server's default broadcaster. | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                        | Req'd | Description                       | Constraints |
| -------- | ------------------------------------------- | ----- | --------------------------------- | ----------- |
| `assets` | Array of [`#brandingAsset`](#brandingasset) | ✅    | List of available branding assets |             |

---

<a name="brandingasset"></a>

### `brandingAsset`

**Type:** `object`

**Properties:**

| Name       | Type     | Req'd | Description                              | Constraints |
| ---------- | -------- | ----- | ---------------------------------------- | ----------- |
| `key`      | `string` | ✅    | Asset key identifier                     |             |
| `mimeType` | `string` | ✅    | MIME type of the asset                   |             |
| `url`      | `string` | ❌    | URL to fetch the asset blob (for images) |             |
| `data`     | `string` | ❌    | Inline data for text assets              |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.getBranding",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get all branding configuration for the broadcaster.",
      "parameters": {
        "type": "params",
        "required": [],
        "properties": {
          "broadcaster": {
            "type": "string",
            "format": "did",
            "description": "DID of the broadcaster. If not provided, uses the server's default broadcaster."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["assets"],
          "properties": {
            "assets": {
              "type": "array",
              "description": "List of available branding assets",
              "items": {
                "type": "ref",
                "ref": "#brandingAsset"
              }
            }
          }
        }
      },
      "errors": []
    },
    "brandingAsset": {
      "type": "object",
      "required": ["key", "mimeType"],
      "properties": {
        "key": {
          "type": "string",
          "description": "Asset key identifier"
        },
        "mimeType": {
          "type": "string",
          "description": "MIME type of the asset"
        },
        "url": {
          "type": "string",
          "description": "URL to fetch the asset blob (for images)"
        },
        "data": {
          "type": "string",
          "description": "Inline data for text assets"
        }
      }
    }
  }
}
```
