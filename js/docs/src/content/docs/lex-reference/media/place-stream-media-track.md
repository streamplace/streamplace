---
title: place.stream.media.track
description: Reference for the place.stream.media.track lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A track for a video stream, either part of the source or a custom additional track.

**Record Key:** `tid`

**Record Properties:**

| Name          | Type                                                                                                                                   | Req'd | Description                                                                               | Constraints |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------------------------------------------------------------------------------------- | ----------- |
| `track`       | Union of:<br/>&nbsp;&nbsp;[`place.stream.media.defs#muxlTrack`](/lex-reference/place-stream-media-defs#muxltrack)                      | ✅    |                                                                                           |             |
| `video`       | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | If this is a derived track like a transcode or a transcript, what video did it come from? |             |
| `parentTrack` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | If this is a derived track like a transcode or a transcript, what was the parent track?   |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.track",
  "defs": {
    "main": {
      "type": "record",
      "description": "A track for a video stream, either part of the source or a custom additional track.",
      "key": "tid",
      "record": {
        "required": ["track"],
        "type": "object",
        "properties": {
          "track": {
            "type": "union",
            "refs": ["place.stream.media.defs#muxlTrack"]
          },
          "video": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "If this is a derived track like a transcode or a transcript, what video did it come from?"
          },
          "parentTrack": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "If this is a derived track like a transcode or a transcript, what was the parent track?"
          }
        }
      }
    }
  }
}
```
