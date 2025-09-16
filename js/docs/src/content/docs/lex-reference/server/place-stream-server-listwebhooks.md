---
title: place.stream.server.listWebhooks
description: Reference for the place.stream.server.listWebhooks lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

List webhooks for the authenticated user.

**Parameters:**

| Name     | Type      | Req'd | Description                                  | Constraints                                     |
| -------- | --------- | ----- | -------------------------------------------- | ----------------------------------------------- |
| `limit`  | `integer` | ❌    | The number of webhooks to return.            | Min: 1<br/>Max: 100<br/>Default: `50`           |
| `cursor` | `string`  | ❌    | An optional cursor for pagination.           |                                                 |
| `active` | `boolean` | ❌    | Filter webhooks by active status.            |                                                 |
| `event`  | `string`  | ❌    | Filter webhooks that handle this event type. | Enum: `chat`, `livestream`, `follow`, `mention` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                                                                                           | Req'd | Description                                         | Constraints |
| ---------- | ---------------------------------------------------------------------------------------------- | ----- | --------------------------------------------------- | ----------- |
| `webhooks` | Array of [`place.stream.server.defs#webhook`](/lex-reference/place-stream-server-defs#webhook) | ✅    |                                                     |             |
| `cursor`   | `string`                                                                                       | ❌    | A cursor for pagination, if there are more results. |             |

**Possible Errors:**

- `InvalidCursor`: The provided cursor is invalid or expired.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.listWebhooks",
  "defs": {
    "main": {
      "type": "query",
      "description": "List webhooks for the authenticated user.",
      "parameters": {
        "type": "params",
        "properties": {
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 50,
            "description": "The number of webhooks to return."
          },
          "cursor": {
            "type": "string",
            "description": "An optional cursor for pagination."
          },
          "active": {
            "type": "boolean",
            "description": "Filter webhooks by active status."
          },
          "event": {
            "type": "string",
            "enum": ["chat", "livestream", "follow", "mention"],
            "description": "Filter webhooks that handle this event type."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["webhooks"],
          "properties": {
            "webhooks": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.server.defs#webhook"
              }
            },
            "cursor": {
              "type": "string",
              "description": "A cursor for pagination, if there are more results."
            }
          }
        }
      },
      "errors": [
        {
          "name": "InvalidCursor",
          "description": "The provided cursor is invalid or expired."
        }
      ]
    }
  }
}
```
