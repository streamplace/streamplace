---
title: place.stream.branding.deleteBlob
description: Reference for the place.stream.branding.deleteBlob lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete a branding asset blob. Requires admin authorization.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type     | Req'd | Description                                                                     | Constraints   |
| ------------- | -------- | ----- | ------------------------------------------------------------------------------- | ------------- |
| `key`         | `string` | ✅    | Branding asset key (mainLogo, favicon, siteTitle, etc.)                         |               |
| `broadcaster` | `string` | ❌    | DID of the broadcaster. If not provided, uses the server's default broadcaster. | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description | Constraints |
| --------- | --------- | ----- | ----------- | ----------- |
| `success` | `boolean` | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`: The authenticated DID is not authorized to modify branding
- `BrandingNotFound`: The requested branding asset does not exist

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.deleteBlob",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete a branding asset blob. Requires admin authorization.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
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
          "name": "BrandingNotFound",
          "description": "The requested branding asset does not exist"
        }
      ]
    }
  }
}
```
