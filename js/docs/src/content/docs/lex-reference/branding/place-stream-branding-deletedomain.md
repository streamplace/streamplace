---
title: place.stream.branding.deleteDomain
description: Reference for the place.stream.branding.deleteDomain lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Stop serving a custom domain. Admins, or the domain's owner.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description | Constraints |
| ---------- | -------- | ----- | ----------- | ----------- |
| `hostname` | `string` | ✅    |             |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description | Constraints |
| --------- | --------- | ----- | ----------- | ----------- |
| `success` | `boolean` | ✅    |             |             |

**Possible Errors:**

- `Unauthorized`
- `DomainNotFound`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.deleteDomain",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Stop serving a custom domain. Admins, or the domain's owner.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["hostname"],
          "properties": {
            "hostname": {
              "type": "string"
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
          "name": "Unauthorized"
        },
        {
          "name": "DomainNotFound"
        }
      ]
    }
  }
}
```
