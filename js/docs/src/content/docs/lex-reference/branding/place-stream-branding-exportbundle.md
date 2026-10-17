---
title: place.stream.branding.exportBundle
description: Reference for the place.stream.branding.exportBundle lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Download the node's branding as a bundle: a zip with branding.yaml (every set key; image keys name files in the zip) plus the image files. Re-import it on another node with place.stream.branding.importBundle. Requires admin authorization.

**Parameters:**

| Name          | Type     | Req'd | Description                                      | Constraints   |
| ------------- | -------- | ----- | ------------------------------------------------ | ------------- |
| `broadcaster` | `string` | ❌    | DID of the broadcaster. Defaults to this node's. | Format: `did` |

**Output:**

- **Encoding:** `application/zip`
- **Schema:**

_Schema not defined._
**Possible Errors:**

- `Unauthorized`: The caller is not an admin.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.exportBundle",
  "defs": {
    "main": {
      "type": "query",
      "description": "Download the node's branding as a bundle: a zip with branding.yaml (every set key; image keys name files in the zip) plus the image files. Re-import it on another node with place.stream.branding.importBundle. Requires admin authorization.",
      "parameters": {
        "type": "params",
        "properties": {
          "broadcaster": {
            "type": "string",
            "format": "did",
            "description": "DID of the broadcaster. Defaults to this node's."
          }
        }
      },
      "output": {
        "encoding": "application/zip"
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The caller is not an admin."
        }
      ]
    }
  }
}
```
