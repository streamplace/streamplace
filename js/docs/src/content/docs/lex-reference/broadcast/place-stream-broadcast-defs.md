---
title: place.stream.broadcast.defs
description: Reference for the place.stream.broadcast.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="broadcastoriginview"></a>

### `broadcastOriginView`

**Type:** `object`

**Properties:**

| Name     | Type                                                                                                                                             | Req'd | Description | Constraints      |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | ----------- | ---------------- |
| `uri`    | `string`                                                                                                                                         | ✅    |             | Format: `at-uri` |
| `cid`    | `string`                                                                                                                                         | ✅    |             | Format: `cid`    |
| `author` | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |             |                  |
| `record` | `unknown`                                                                                                                                        | ✅    |             |                  |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.broadcast.defs",
  "defs": {
    "broadcastOriginView": {
      "type": "object",
      "required": ["uri", "cid", "author", "record"],
      "properties": {
        "uri": {
          "type": "string",
          "format": "at-uri"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "author": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic"
        },
        "record": {
          "type": "unknown"
        }
      }
    }
  }
}
```
