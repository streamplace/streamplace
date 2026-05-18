---
title: place.stream.media.track
description: Reference for the place.stream.media.track lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A track for a video stream, either part of the source or a custom additional track. One of: video, audio, subtitles.

**Record Key:** `tid`

**Record Properties:**

| Name          | Type                                                                                                                                   | Req'd | Description                                                                                          | Constraints      |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ---------------------------------------------------------------------------------------------------- | ---------------- |
| `track`       | Union of:<br/>&nbsp;&nbsp;[`place.stream.media.defs#muxlTrack`](/lex-reference/place-stream-media-defs#muxltrack)                      | ✅    |                                                                                                      |                  |
| `video`       | `string`                                                                                                                               | ❌    | If this is a derived track like a transcode or a transcript, what was the source place.stream.video? | Format: `at-uri` |
| `parentTrack` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | If this is a derived track like a transcode or a transcript, what was the parent track?              |                  |
| `metadata`    | Union of:<br/>&nbsp;&nbsp;[`place.stream.media.track#commonMetadata`](/lex-reference/place-stream-media-track#commonmetadata)          | ❌    |                                                                                                      |                  |

---

<a name="commonmetadata"></a>

### `commonMetadata`

**Type:** `object`

Metadata common to all media types. Contains subobjects for other media types.

**Properties:**

| Name         | Type                                                                      | Req'd | Description                                                         | Constraints |
| ------------ | ------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------- | ----------- |
| `language`   | `string`                                                                  | ❌    | IETF BCP 47 language tag corresponding to the content of this track |             |
| `durationMs` | `integer`                                                                 | ❌    | duration of this track in milliseconds                              |             |
| `video`      | [`place.stream.segment#video`](/lex-reference/place-stream-segment#video) | ❌    |                                                                     |             |
| `audio`      | [`place.stream.segment#audio`](/lex-reference/place-stream-segment#audio) | ❌    |                                                                     |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.track",
  "defs": {
    "main": {
      "type": "record",
      "description": "A track for a video stream, either part of the source or a custom additional track. One of: video, audio, subtitles.",
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
            "type": "string",
            "format": "at-uri",
            "description": "If this is a derived track like a transcode or a transcript, what was the source place.stream.video?"
          },
          "parentTrack": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "If this is a derived track like a transcode or a transcript, what was the parent track?"
          },
          "metadata": {
            "type": "union",
            "refs": ["place.stream.media.track#commonMetadata"]
          }
        }
      }
    },
    "commonMetadata": {
      "type": "object",
      "description": "Metadata common to all media types. Contains subobjects for other media types.",
      "required": [],
      "properties": {
        "language": {
          "type": "string",
          "description": "IETF BCP 47 language tag corresponding to the content of this track"
        },
        "durationMs": {
          "type": "integer",
          "description": "duration of this track in milliseconds"
        },
        "video": {
          "type": "ref",
          "ref": "place.stream.segment#video"
        },
        "audio": {
          "type": "ref",
          "ref": "place.stream.segment#audio"
        }
      }
    }
  }
}
```
