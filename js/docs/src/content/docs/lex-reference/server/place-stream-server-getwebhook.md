---
title: place.stream.server.getWebhook
description: Reference for the place.stream.server.getWebhook lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get details for a specific webhook.

**Parameters:**

| Name | Type     | Req'd | Description                        | Constraints |
| ---- | -------- | ----- | ---------------------------------- | ----------- |
| `id` | `string` | ✅    | The ID of the webhook to retrieve. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                                  | Req'd | Description | Constraints |
| --------- | ------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `webhook` | [`place.stream.server.defs#webhook`](/lex-reference/place-stream-server-defs#webhook) | ✅    |             |             |

**Possible Errors:**

- `WebhookNotFound`: The specified webhook was not found.
- `Unauthorized`: The authenticated user does not have access to this webhook.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.getWebhook",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get details for a specific webhook.",
      "parameters": {
        "type": "params",
        "required": ["id"],
        "properties": {
          "id": {
            "type": "string",
            "description": "The ID of the webhook to retrieve."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["webhook"],
          "properties": {
            "webhook": {
              "type": "ref",
              "ref": "place.stream.server.defs#webhook"
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
