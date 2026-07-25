---
title: place.stream.badge.def
description: Reference for the place.stream.badge.def lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Defines a badge's visual appearance and type. Created by the issuer and referenced by issuance records.

**Record Key:** `tid`

**Record Properties:**

| Name          | Type     | Req'd | Description                                                   | Constraints                                                                             |
| ------------- | -------- | ----- | ------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `name`        | `string` | ✅    | Display name for this badge.                                  | Max Length: 64                                                                          |
| `description` | `string` | ❌    | Optional description of the badge.                            | Max Length: 256                                                                         |
| `image`       | `blob`   | ❌    | Badge icon image.                                             | Accept: `image/png`, `image/jpeg`, `image/gif`, `image/webp`<br/>Max Size: 262144 bytes |
| `badgeType`   | `string` | ✅    | The category of this badge, used for scope and display rules. | Known Values: `place.stream.badge.defs#vip`, `place.stream.badge.defs#event`            |
| `createdAt`   | `string` | ✅    |                                                               | Format: `datetime`                                                                      |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.def",
  "defs": {
    "main": {
      "type": "record",
      "description": "Defines a badge's visual appearance and type. Created by the issuer and referenced by issuance records.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["name", "badgeType", "createdAt"],
        "properties": {
          "name": {
            "type": "string",
            "maxLength": 64,
            "description": "Display name for this badge."
          },
          "description": {
            "type": "string",
            "maxLength": 256,
            "description": "Optional description of the badge."
          },
          "image": {
            "type": "blob",
            "accept": ["image/png", "image/jpeg", "image/gif", "image/webp"],
            "maxSize": 262144,
            "description": "Badge icon image."
          },
          "badgeType": {
            "type": "string",
            "knownValues": [
              "place.stream.badge.defs#vip",
              "place.stream.badge.defs#event"
            ],
            "description": "The category of this badge, used for scope and display rules."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime"
          }
        }
      }
    }
  }
}
```
