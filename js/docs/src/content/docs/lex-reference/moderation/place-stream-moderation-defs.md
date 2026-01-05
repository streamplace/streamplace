---
title: place.stream.moderation.defs
description: Reference for the place.stream.moderation.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="permissionview"></a>

### `permissionView`

**Type:** `object`

**Properties:**

| Name     | Type                                                                                                                                             | Req'd | Description                                 | Constraints      |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ------------------------------------------- | ---------------- |
| `uri`    | `string`                                                                                                                                         | ✅    | AT-URI of the permission record             | Format: `at-uri` |
| `cid`    | `string`                                                                                                                                         | ✅    | Content identifier of the permission record | Format: `cid`    |
| `author` | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    | The streamer who granted these permissions  |                  |
| `record` | `unknown`                                                                                                                                        | ✅    | The permission record itself                |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.defs",
  "defs": {
    "permissionView": {
      "type": "object",
      "required": ["uri", "cid", "author", "record"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri",
          "description": "AT-URI of the permission record"
        },
        "cid": {
          "type": "string",
          "format": "cid",
          "description": "Content identifier of the permission record"
        },
        "author": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic",
          "description": "The streamer who granted these permissions"
        },
        "record": {
          "type": "unknown",
          "description": "The permission record itself"
        }
      }
    }
  }
}
```
