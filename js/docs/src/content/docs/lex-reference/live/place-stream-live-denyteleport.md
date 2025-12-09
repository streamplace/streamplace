---
title: place.stream.live.denyTeleport
description: Reference for the place.stream.live.denyTeleport lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Deny an incoming teleport request.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name  | Type     | Req'd | Description                             | Constraints      |
| ----- | -------- | ----- | --------------------------------------- | ---------------- |
| `uri` | `string` | ✅    | The URI of the teleport record to deny. | Format: `at-uri` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description                                   | Constraints |
| --------- | --------- | ----- | --------------------------------------------- | ----------- |
| `success` | `boolean` | ✅    | Whether the teleport was successfully denied. |             |

**Possible Errors:**

- `TeleportNotFound`: The specified teleport was not found.
- `Unauthorized`: The authenticated user is not the target of this teleport.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.live.denyTeleport",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Deny an incoming teleport request.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri"],
          "properties": {
            "uri": {
              "type": "string",
              "format": "at-uri",
              "description": "The URI of the teleport record to deny."
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
              "type": "boolean",
              "description": "Whether the teleport was successfully denied."
            }
          }
        }
      },
      "errors": [
        {
          "name": "TeleportNotFound",
          "description": "The specified teleport was not found."
        },
        {
          "name": "Unauthorized",
          "description": "The authenticated user is not the target of this teleport."
        }
      ]
    }
  }
}
```
