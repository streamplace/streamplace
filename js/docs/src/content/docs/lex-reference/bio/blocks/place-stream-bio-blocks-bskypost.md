---
title: place.stream.bio.blocks.bskyPost
description: Reference for the place.stream.bio.blocks.bskyPost lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

An embedded reference to a Bluesky post.

**Properties:**

| Name   | Type                                                                                                                                   | Req'd | Description | Constraints |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `post` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.bskyPost",
  "defs": {
    "main": {
      "type": "object",
      "description": "An embedded reference to a Bluesky post.",
      "required": ["post"],
      "properties": {
        "post": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef"
        }
      }
    }
  }
}
```
