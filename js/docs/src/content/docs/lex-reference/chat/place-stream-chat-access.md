---
title: place.stream.chat.access
description: Reference for the place.stream.chat.access lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A rule about who may chat on this streamer's streams. Rules are evaluated per streamer: when any allow rule exists, only accounts matching an allow rule may chat; accounts matching a deny rule never may. A streamer with no rules has open chat.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                                  | Req'd | Description                                                                                     | Constraints           |
| ----------- | ------------------------------------------------------------------------------------- | ----- | ----------------------------------------------------------------------------------------------- | --------------------- |
| `action`    | `string`                                                                              | ✅    | Whether accounts matching the subject may chat (allow) or may not (deny). Deny wins over allow. | Enum: `allow`, `deny` |
| `subject`   | Union of:<br/>&nbsp;&nbsp;[`#verifier`](#verifier)<br/>&nbsp;&nbsp;[`#label`](#label) | ✅    | Who the rule matches. Open: a node ignores subject types it does not understand.                |                       |
| `createdAt` | `string`                                                                              | ✅    | Client-declared timestamp when this rule was created.                                           | Format: `datetime`    |

---

<a name="verifier"></a>

### `verifier`

**Type:** `object`

Accounts that hold an app.bsky.graph.verification record issued by this DID. A node that sees a rule naming a verifier starts indexing that verifier's records.

**Properties:**

| Name  | Type     | Req'd | Description         | Constraints   |
| ----- | -------- | ----- | ------------------- | ------------- |
| `did` | `string` | ✅    | The verifier's DID. | Format: `did` |

---

<a name="label"></a>

### `label`

**Type:** `object`

Accounts carrying this label from this labeler. A node that sees a rule naming a labeler starts mirroring its matching labels.

**Properties:**

| Name      | Type     | Req'd | Description                                                                     | Constraints     |
| --------- | -------- | ----- | ------------------------------------------------------------------------------- | --------------- |
| `labeler` | `string` | ✅    | The labeler's DID.                                                              | Format: `did`   |
| `value`   | `string` | ✅    | The label value that matches; a trailing \* matches any label with that prefix. | Max Length: 128 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.chat.access",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "A rule about who may chat on this streamer's streams. Rules are evaluated per streamer: when any allow rule exists, only accounts matching an allow rule may chat; accounts matching a deny rule never may. A streamer with no rules has open chat.",
      "record": {
        "type": "object",
        "required": ["action", "subject", "createdAt"],
        "properties": {
          "action": {
            "type": "string",
            "enum": ["allow", "deny"],
            "description": "Whether accounts matching the subject may chat (allow) or may not (deny). Deny wins over allow."
          },
          "subject": {
            "type": "union",
            "refs": ["#verifier", "#label"],
            "description": "Who the rule matches. Open: a node ignores subject types it does not understand."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this rule was created."
          }
        }
      }
    },
    "verifier": {
      "type": "object",
      "description": "Accounts that hold an app.bsky.graph.verification record issued by this DID. A node that sees a rule naming a verifier starts indexing that verifier's records.",
      "required": ["did"],
      "properties": {
        "did": {
          "type": "string",
          "format": "did",
          "description": "The verifier's DID."
        }
      }
    },
    "label": {
      "type": "object",
      "description": "Accounts carrying this label from this labeler. A node that sees a rule naming a labeler starts mirroring its matching labels.",
      "required": ["labeler", "value"],
      "properties": {
        "labeler": {
          "type": "string",
          "format": "did",
          "description": "The labeler's DID."
        },
        "value": {
          "type": "string",
          "maxLength": 128,
          "description": "The label value that matches; a trailing * matches any label with that prefix."
        }
      }
    }
  }
}
```
