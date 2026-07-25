---
title: place.stream.badge.issuance
description: Reference for the place.stream.badge.issuance lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Grants a specific badge to a recipient. The badge only appears in chat after the recipient adds this record to their place.stream.chat.profile selection array.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                                                                                   | Req'd | Description                                                          | Constraints        |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------- | ------------------ |
| `did`       | `string`                                                                                                                               | ✅    | The DID of the user being granted the badge.                         | Format: `did`      |
| `badge`     | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | Strong reference to the place.stream.badge.def record being granted. |                    |
| `createdAt` | `string`                                                                                                                               | ✅    | Client-declared timestamp when this issuance was created.            | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.issuance",
  "defs": {
    "main": {
      "type": "record",
      "description": "Grants a specific badge to a recipient. The badge only appears in chat after the recipient adds this record to their place.stream.chat.profile selection array.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["did", "badge", "createdAt"],
        "properties": {
          "did": {
            "type": "string",
            "format": "did",
            "description": "The DID of the user being granted the badge."
          },
          "badge": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "Strong reference to the place.stream.badge.def record being granted."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this issuance was created."
          }
        }
      }
    }
  }
}
```
