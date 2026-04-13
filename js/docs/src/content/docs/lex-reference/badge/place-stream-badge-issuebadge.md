---
title: place.stream.badge.issueBadge
description: Reference for the place.stream.badge.issueBadge lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create a badge definition and grant it to a recipient. Both records are written to the authenticated user's repo.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name           | Type     | Req'd | Description                              | Constraints                                                                  |
| -------------- | -------- | ----- | ---------------------------------------- | ---------------------------------------------------------------------------- |
| `recipientDid` | `string` | ✅    | The DID of the user receiving the badge. | Format: `did`                                                                |
| `name`         | `string` | ✅    | Display name for the badge.              | Max Length: 64                                                               |
| `description`  | `string` | ❌    | Optional description of the badge.       | Max Length: 256                                                              |
| `badgeType`    | `string` | ✅    | The category of badge being issued.      | Known Values: `place.stream.badge.defs#vip`, `place.stream.badge.defs#event` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type     | Req'd | Description                                    | Constraints      |
| ------------- | -------- | ----- | ---------------------------------------------- | ---------------- |
| `defUri`      | `string` | ✅    | AT URI of the created badge definition record. | Format: `at-uri` |
| `defCid`      | `string` | ✅    | CID of the created badge definition record.    |                  |
| `issuanceUri` | `string` | ✅    | AT URI of the created badge issuance record.   | Format: `at-uri` |
| `issuanceCid` | `string` | ✅    | CID of the created badge issuance record.      |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.issueBadge",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create a badge definition and grant it to a recipient. Both records are written to the authenticated user's repo.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["recipientDid", "name", "badgeType"],
          "properties": {
            "recipientDid": {
              "type": "string",
              "format": "did",
              "description": "The DID of the user receiving the badge."
            },
            "name": {
              "type": "string",
              "maxLength": 64,
              "description": "Display name for the badge."
            },
            "description": {
              "type": "string",
              "maxLength": 256,
              "description": "Optional description of the badge."
            },
            "badgeType": {
              "type": "string",
              "knownValues": [
                "place.stream.badge.defs#vip",
                "place.stream.badge.defs#event"
              ],
              "description": "The category of badge being issued."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["defUri", "defCid", "issuanceUri", "issuanceCid"],
          "properties": {
            "defUri": {
              "type": "string",
              "format": "at-uri",
              "description": "AT URI of the created badge definition record."
            },
            "defCid": {
              "type": "string",
              "description": "CID of the created badge definition record."
            },
            "issuanceUri": {
              "type": "string",
              "format": "at-uri",
              "description": "AT URI of the created badge issuance record."
            },
            "issuanceCid": {
              "type": "string",
              "description": "CID of the created badge issuance record."
            }
          }
        }
      }
    }
  }
}
```
