---
title: place.stream.muxl.catalog
description: Reference for the place.stream.muxl.catalog lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Track configuration for a MUXL stream. Contains the catalog (codecs, dimensions, timescales) and per-track init segments. A new catalog record is created whenever the track configuration changes (e.g. resolution change at a keyframe).

**Record Key:** `tid`

**Record Properties:**

| Name          | Type                                                                                                                                   | Req'd | Description                                           | Constraints         |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------------------------------------------------- | ------------------- |
| `video`       | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | The place.stream.video this catalog belongs to.       |                     |
| `catalog`     | [`place.stream.video#catalog`](/lex-reference/place-stream-video#catalog)                                                              | ✅    | Track configuration metadata.                         |                     |
| `initSegment` | `blob`                                                                                                                                 | ✅    | The multi-track ftyp+moov init segment.               | Accept: `video/mp4` |
| `trackInits`  | Array of [`#trackInit`](#trackinit)                                                                                                    | ✅    | Per-track init segments for HLS CMAF media playlists. |                     |

---

<a name="trackinit"></a>

### `trackInit`

**Type:** `object`

**Properties:**

| Name      | Type      | Req'd | Description                          | Constraints         |
| --------- | --------- | ----- | ------------------------------------ | ------------------- |
| `trackId` | `integer` | ✅    | MP4 track ID.                        |                     |
| `data`    | `blob`    | ✅    | Single-track ftyp+moov init segment. | Accept: `video/mp4` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.muxl.catalog",
  "defs": {
    "main": {
      "type": "record",
      "description": "Track configuration for a MUXL stream. Contains the catalog (codecs, dimensions, timescales) and per-track init segments. A new catalog record is created whenever the track configuration changes (e.g. resolution change at a keyframe).",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["video", "catalog", "initSegment", "trackInits"],
        "properties": {
          "video": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The place.stream.video this catalog belongs to."
          },
          "catalog": {
            "type": "ref",
            "ref": "place.stream.video#catalog",
            "description": "Track configuration metadata."
          },
          "initSegment": {
            "type": "blob",
            "accept": ["video/mp4"],
            "description": "The multi-track ftyp+moov init segment."
          },
          "trackInits": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "#trackInit"
            },
            "description": "Per-track init segments for HLS CMAF media playlists."
          }
        }
      }
    },
    "trackInit": {
      "type": "object",
      "required": ["trackId", "data"],
      "properties": {
        "trackId": {
          "type": "integer",
          "description": "MP4 track ID."
        },
        "data": {
          "type": "blob",
          "accept": ["video/mp4"],
          "description": "Single-track ftyp+moov init segment."
        }
      }
    }
  }
}
```
