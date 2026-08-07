---
title: place.stream.clip.gate
description: Reference for the place.stream.clip.gate lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record defining a gated (hidden) clip. Created in the streamer's repository by a moderator with the 'clip.hide' permission. When present, the referenced clip is hidden from display on this node.

**Record Key:** `tid`

**Record Properties:**

| Name         | Type                                                                                                                                   | Req'd | Description                                      | Constraints |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------ | ----------- |
| `hiddenClip` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | The place.stream.clip.entry record being hidden. |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.clip.gate",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record defining a gated (hidden) clip. Created in the streamer's repository by a moderator with the 'clip.hide' permission. When present, the referenced clip is hidden from display on this node.",
      "record": {
        "type": "object",
        "required": ["hiddenClip"],
        "properties": {
          "hiddenClip": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The place.stream.clip.entry record being hidden."
          }
        }
      }
    }
  }
}
```
