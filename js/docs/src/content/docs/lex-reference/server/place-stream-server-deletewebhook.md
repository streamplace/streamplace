---
title: place.stream.server.deleteWebhook
description: Reference for the place.stream.server.deleteWebhook lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Delete an existing webhook.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name | Type     | Req'd | Description                      | Constraints |
| ---- | -------- | ----- | -------------------------------- | ----------- |
| `id` | `string` | ✅    | The ID of the webhook to delete. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type      | Req'd | Description                                   | Constraints |
| --------- | --------- | ----- | --------------------------------------------- | ----------- |
| `success` | `boolean` | ✅    | Whether the webhook was successfully deleted. |             |

**Possible Errors:**

- `WebhookNotFound`: The specified webhook was not found.
- `Unauthorized`: The authenticated user does not have access to this webhook.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.deleteWebhook",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Delete an existing webhook.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["id"],
          "properties": {
            "id": {
              "type": "string",
              "description": "The ID of the webhook to delete."
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
              "description": "Whether the webhook was successfully deleted."
            }
          }
        }
      },
      "errors": [
        {
          "name": "WebhookNotFound",
          "description": "The specified webhook was not found."
        },
        {
          "name": "Unauthorized",
          "description": "The authenticated user does not have access to this webhook."
        }
      ]
    }
  }
}
```
