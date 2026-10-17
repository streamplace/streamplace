---
title: place.stream.branding.importBundle
description: Reference for the place.stream.branding.importBundle lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Apply a branding bundle (see exportBundle). By default the bundle replaces the node's branding: keys it does not mention are removed. Every value is validated like updateBlob. Requires admin authorization.

**Parameters:**

| Name          | Type      | Req'd | Description                                                     | Constraints      |
| ------------- | --------- | ----- | --------------------------------------------------------------- | ---------------- |
| `broadcaster` | `string`  | ❌    | DID of the broadcaster. Defaults to this node's.                | Format: `did`    |
| `merge`       | `boolean` | ❌    | Keep keys the bundle does not mention instead of removing them. | Default: `false` |
| `dryRun`      | `boolean` | ❌    | Report what would change without writing anything.              | Default: `false` |

**Input:**

- **Encoding:** `application/zip`
- **Schema:**

_Schema not defined._
**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                          | Req'd | Description          | Constraints |
| ---------- | ----------------------------- | ----- | -------------------- | ----------- |
| `applied`  | `boolean`                     | ✅    | False for a dry run. |             |
| `changes`  | Array of [`#change`](#change) | ✅    |                      |             |
| `warnings` | Array of `string`             | ❌    |                      |             |

**Possible Errors:**

- `Unauthorized`: The caller is not an admin.
- `InvalidBundle`: The zip, its branding.yaml or one of its values is not acceptable; the message names the key.

---

<a name="change"></a>

### `change`

**Type:** `object`

**Properties:**

| Name     | Type     | Req'd | Description                                       | Constraints                                              |
| -------- | -------- | ----- | ------------------------------------------------- | -------------------------------------------------------- |
| `key`    | `string` | ✅    |                                                   |                                                          |
| `action` | `string` | ✅    |                                                   | Known Values: `added`, `changed`, `removed`, `unchanged` |
| `detail` | `string` | ❌    | The new text value, or the image's type and size. |                                                          |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.importBundle",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Apply a branding bundle (see exportBundle). By default the bundle replaces the node's branding: keys it does not mention are removed. Every value is validated like updateBlob. Requires admin authorization.",
      "parameters": {
        "type": "params",
        "properties": {
          "broadcaster": {
            "type": "string",
            "format": "did",
            "description": "DID of the broadcaster. Defaults to this node's."
          },
          "merge": {
            "type": "boolean",
            "default": false,
            "description": "Keep keys the bundle does not mention instead of removing them."
          },
          "dryRun": {
            "type": "boolean",
            "default": false,
            "description": "Report what would change without writing anything."
          }
        }
      },
      "input": {
        "encoding": "application/zip"
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["applied", "changes"],
          "properties": {
            "applied": {
              "type": "boolean",
              "description": "False for a dry run."
            },
            "changes": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "#change"
              }
            },
            "warnings": {
              "type": "array",
              "items": {
                "type": "string"
              }
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The caller is not an admin."
        },
        {
          "name": "InvalidBundle",
          "description": "The zip, its branding.yaml or one of its values is not acceptable; the message names the key."
        }
      ]
    },
    "change": {
      "type": "object",
      "required": ["key", "action"],
      "properties": {
        "key": {
          "type": "string"
        },
        "action": {
          "type": "string",
          "knownValues": ["added", "changed", "removed", "unchanged"]
        },
        "detail": {
          "type": "string",
          "description": "The new text value, or the image's type and size."
        }
      }
    }
  }
}
```
