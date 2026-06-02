---
title: place.stream.moderation.permission
description: Reference for the place.stream.moderation.permission lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record granting moderation permissions to a user for this streamer's content.

**Record Key:** `tid`

**Record Properties:**

| Name             | Type              | Req'd | Description                                                                                                                                                                                | Constraints        |
| ---------------- | ----------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------ |
| `moderator`      | `string`          | ✅    | The DID of the user granted moderator permissions.                                                                                                                                         | Format: `did`      |
| `permissions`    | Array of `string` | ✅    | Array of permissions granted to this moderator. 'ban' covers blocks/bans (with optional expiration), 'hide' covers message gates, 'livestream.manage' allows updating livestream metadata. |                    |
| `createdAt`      | `string`          | ✅    | Client-declared timestamp when this moderator was added.                                                                                                                                   | Format: `datetime` |
| `expirationTime` | `string`          | ❌    | Optional expiration time for this delegation. If set, the delegation is invalid after this time.                                                                                           | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.permission",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record granting moderation permissions to a user for this streamer's content.",
      "record": {
        "type": "object",
        "required": ["moderator", "permissions", "createdAt"],
        "properties": {
          "moderator": {
            "type": "string",
            "format": "did",
            "description": "The DID of the user granted moderator permissions."
          },
          "permissions": {
            "type": "array",
            "items": {
              "type": "string",
              "enum": [
                "ban",
                "hide",
                "livestream.manage",
                "message.pin",
                "vod.comment.hide"
              ]
            },
            "description": "Array of permissions granted to this moderator. 'ban' covers blocks/bans (with optional expiration), 'hide' covers message gates, 'livestream.manage' allows updating livestream metadata."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this moderator was added."
          },
          "expirationTime": {
            "type": "string",
            "format": "datetime",
            "description": "Optional expiration time for this delegation. If set, the delegation is invalid after this time."
          }
        }
      }
    }
  }
}
```
