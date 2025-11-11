---
title: place.stream.branding.updateBlob
description: Reference for the place.stream.branding.updateBlob lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Update or create a branding asset blob. Requires admin authorization.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type      | Req'd | Description                                                                     | Constraints   |
| ------------- | --------- | ----- | ------------------------------------------------------------------------------- | ------------- |
| `key`         | `string`  | ✅    | Branding asset key (mainLogo, favicon, siteTitle, etc.)                         |               |
| `broadcaster` | `string`  | ❌    | DID of the broadcaster. If not provided, uses the server's default broadcaster. | Format: `did` |
| `data`        | `string`  | ✅    | Base64-encoded blob data                                                        |               |
| `mimeType`    | `string`  | ✅    | MIME type of the blob (e.g., image/png, text/plain)                             |               |
| `width`       | `integer` | ❌    | Image width in pixels (optional, for images only)                               |               |
| `height`      | `integer` | ❌    | Image height in pixels (optional, for images only)                              |               |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description | Constraints |
| --------- | --------- | ----- | ----------- | ----------- |
| `success` | `boolean` | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`: The authenticated DID is not authorized to modify branding
- `BlobTooLarge`: The blob exceeds the maximum size limit

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.updateBlob",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Update or create a branding asset blob. Requires admin authorization.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["key", "data", "mimeType"],
          "properties": {
            "key": {
              "type": "string",
              "description": "Branding asset key (mainLogo, favicon, siteTitle, etc.)"
            },
            "broadcaster": {
              "type": "string",
              "format": "did",
              "description": "DID of the broadcaster. If not provided, uses the server's default broadcaster."
            },
            "data": {
              "type": "string",
              "description": "Base64-encoded blob data"
            },
            "mimeType": {
              "type": "string",
              "description": "MIME type of the blob (e.g., image/png, text/plain)"
            },
            "width": {
              "type": "integer",
              "description": "Image width in pixels (optional, for images only)"
            },
            "height": {
              "type": "integer",
              "description": "Image height in pixels (optional, for images only)"
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["success"],
          "properties": {
            "success": {
              "type": "boolean"
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The authenticated DID is not authorized to modify branding"
        },
        {
          "name": "BlobTooLarge",
          "description": "The blob exceeds the maximum size limit"
        }
      ]
    }
  }
}
```
