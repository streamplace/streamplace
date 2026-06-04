---
title: place.stream.bio.blocks.livestream
description: Reference for the place.stream.bio.blocks.livestream lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

An embedded reference to a place.stream.livestream record. Useful for pinning a current stream or a notable past stream.

**Properties:**

| Name         | Type                                                                                                                                   | Req'd | Description | Constraints |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `livestream` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.blocks.livestream",
  "defs": {
    "main": {
      "type": "object",
      "description": "An embedded reference to a place.stream.livestream record. Useful for pinning a current stream or a notable past stream.",
      "required": ["livestream"],
      "properties": {
        "livestream": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef"
        }
      }
    }
  }
}
```
