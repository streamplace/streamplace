---
title: place.stream.server.defs
description: Reference for the place.stream.server.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="webhook"></a>

### `webhook`

**Type:** `object`

A webhook configuration for receiving Streamplace events.

**Properties:**

| Name            | Type                                    | Req'd | Description                                                                                           | Constraints        |
| --------------- | --------------------------------------- | ----- | ----------------------------------------------------------------------------------------------------- | ------------------ |
| `id`            | `string`                                | ✅    | Unique identifier for this webhook.                                                                   |                    |
| `url`           | `string`                                | ✅    | The webhook URL where events will be sent.                                                            | Format: `uri`      |
| `events`        | Array of `string`                       | ✅    | The types of events this webhook should receive.                                                      |                    |
| `active`        | `boolean`                               | ✅    | Whether this webhook is currently active.                                                             |                    |
| `prefix`        | `string`                                | ❌    | Text to prepend to webhook messages.                                                                  | Max Length: 100    |
| `suffix`        | `string`                                | ❌    | Text to append to webhook messages.                                                                   | Max Length: 100    |
| `rewrite`       | Array of [`#rewriteRule`](#rewriterule) | ❌    | Text replacement rules for webhook messages.                                                          |                    |
| `createdAt`     | `string`                                | ✅    | When this webhook was created.                                                                        | Format: `datetime` |
| `updatedAt`     | `string`                                | ❌    | When this webhook was last updated.                                                                   | Format: `datetime` |
| `name`          | `string`                                | ❌    | A user-friendly name for this webhook.                                                                | Max Length: 100    |
| `description`   | `string`                                | ❌    | A description of what this webhook is used for.                                                       | Max Length: 500    |
| `lastTriggered` | `string`                                | ❌    | When this webhook was last triggered.                                                                 | Format: `datetime` |
| `errorCount`    | `integer`                               | ❌    | Number of consecutive errors for this webhook.                                                        |                    |
| `muteWords`     | Array of `string`                       | ❌    | Words to filter out from chat messages. Messages containing any of these words will not be forwarded. |                    |

---

<a name="rewriterule"></a>

### `rewriteRule`

**Type:** `object`

**Properties:**

| Name   | Type     | Req'd | Description                     | Constraints                       |
| ------ | -------- | ----- | ------------------------------- | --------------------------------- |
| `from` | `string` | ✅    | Text to search for and replace. | Min Length: 1<br/>Max Length: 100 |
| `to`   | `string` | ✅    | Text to replace with.           | Max Length: 100                   |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.defs",
  "defs": {
    "webhook": {
      "type": "object",
      "description": "A webhook configuration for receiving Streamplace events.",
      "required": ["id", "url", "events", "active", "createdAt"],
      "properties": {
        "id": {
          "type": "string",
          "description": "Unique identifier for this webhook."
        },
        "url": {
          "type": "string",
          "format": "uri",
          "description": "The webhook URL where events will be sent."
        },
        "events": {
          "type": "array",
          "items": {
            "type": "string",
            "enum": ["chat", "livestream", "follow", "mention"]
          },
          "description": "The types of events this webhook should receive."
        },
        "active": {
          "type": "boolean",
          "description": "Whether this webhook is currently active."
        },
        "prefix": {
          "type": "string",
          "maxLength": 100,
          "description": "Text to prepend to webhook messages."
        },
        "suffix": {
          "type": "string",
          "maxLength": 100,
          "description": "Text to append to webhook messages."
        },
        "rewrite": {
          "type": "array",
          "items": {
            "type": "ref",
            "ref": "#rewriteRule"
          },
          "description": "Text replacement rules for webhook messages."
        },
        "createdAt": {
          "type": "string",
          "format": "datetime",
          "description": "When this webhook was created."
        },
        "updatedAt": {
          "type": "string",
          "format": "datetime",
          "description": "When this webhook was last updated."
        },
        "name": {
          "type": "string",
          "maxLength": 100,
          "description": "A user-friendly name for this webhook."
        },
        "description": {
          "type": "string",
          "maxLength": 500,
          "description": "A description of what this webhook is used for."
        },
        "lastTriggered": {
          "type": "string",
          "format": "datetime",
          "description": "When this webhook was last triggered."
        },
        "errorCount": {
          "type": "integer",
          "description": "Number of consecutive errors for this webhook."
        },
        "muteWords": {
          "type": "array",
          "items": {
            "type": "string",
            "maxLength": 100
          },
          "description": "Words to filter out from chat messages. Messages containing any of these words will not be forwarded."
        }
      }
    },
    "rewriteRule": {
      "type": "object",
      "required": ["from", "to"],
      "properties": {
        "from": {
          "type": "string",
          "maxLength": 100,
          "minLength": 1,
          "description": "Text to search for and replace."
        },
        "to": {
          "type": "string",
          "maxLength": 100,
          "description": "Text to replace with."
        }
      }
    }
  }
}
```
