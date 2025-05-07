---
title: place.stream.account.defs
description: Reference for the place.stream.account.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="loginresponse"></a>

### `loginResponse`

**Type:** `object`

**Properties:**

| Name          | Type     | Req'd | Description | Constraints   |
| ------------- | -------- | ----- | ----------- | ------------- |
| `redirectUrl` | `string` | ✅    |             | Format: `uri` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.account.defs",
  "defs": {
    "loginResponse": {
      "type": "object",
      "required": ["redirectUrl"],
      "properties": {
        "redirectUrl": {
          "type": "string",
          "format": "uri"
        }
      }
    }
  }
}
```
