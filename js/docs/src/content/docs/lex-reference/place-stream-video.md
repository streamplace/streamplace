---
title: place.stream.video
description: Reference for the place.stream.video lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Some audiovisual content.

**Record Key:** `tid`

**Record Properties:**

| Name                | Type                                                                                                                                                                                                                              | Req'd | Description                                                                                                                                                                                       | Constraints                                   |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `title`             | `string`                                                                                                                                                                                                                          | ✅    | Title of the video referenced by this record                                                                                                                                                      | Max Length: 1400<br/>Max Graphemes: 140       |
| `source`            | Union of:<br/>&nbsp;&nbsp;[`place.stream.media.defs#sourceTracks`](/lex-reference/place-stream-media-defs#sourcetracks)<br/>&nbsp;&nbsp;[`place.stream.media.defs#sourceClip`](/lex-reference/place-stream-media-defs#sourceclip) | ❌    | What is the source of this video?                                                                                                                                                                 |                                               |
| `description`       | `string`                                                                                                                                                                                                                          | ❌    | Description of this video                                                                                                                                                                         | Max Length: 50000<br/>Max Graphemes: 5000     |
| `duration`          | `integer`                                                                                                                                                                                                                         | ❌    | Duration of the video in milliseconds                                                                                                                                                             |                                               |
| `descriptionFacets` | Array of [`place.stream.richtext.facet`](/lex-reference/place-stream-richtext-facet)                                                                                                                                              | ❌    | Annotations of text (mentions, URLs, etc)                                                                                                                                                         |                                               |
| `thumb`             | `blob`                                                                                                                                                                                                                            | ❌    | Thumbnail image for the video.                                                                                                                                                                    | Accept: `image/*`<br/>Max Size: 1000000 bytes |
| `connections`       | Array of Union of:<br/>&nbsp;&nbsp;[`#connection`](#connection)                                                                                                                                                                   | ❌    | Free-form list of atproto records related in some way to this video                                                                                                                               |                                               |
| `contentPolicy`     | [`place.stream.metadata.configuration`](/lex-reference/place-stream-metadata-configuration)                                                                                                                                       | ❌    | copyright, distribution, and content warning data                                                                                                                                                 |                                               |
| `activity`          | Union of:<br/>&nbsp;&nbsp;[`place.stream.defs#activityGame`](/lex-reference/place-stream-defs#activitygame)<br/>&nbsp;&nbsp;[`place.stream.defs#activityLabel`](/lex-reference/place-stream-defs#activitylabel)                   | ❌    | The game or activity in the video.                                                                                                                                                                |                                               |
| `tags`              | Array of `string`                                                                                                                                                                                                                 | ❌    | Freeform tags for this stream. Each tag must be alphanumeric (a-z, A-Z, 0-9) plus colon. Tags with colons indicate a specific tag group (e.g. 'lang:en' indicates the stream's primary language). | Max Items: 10                                 |

---

<a name="connection"></a>

### `connection`

**Type:** `object`

**Properties:**

| Name  | Type                                                                                                                                   | Req'd | Description | Constraints |
| ----- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `ref` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    |             |             |

---

<a name="videosourcecontent"></a>

### `videoSourceContent`

**Type:** `object`

**Properties:**

| Name  | Type                                                                                                                                   | Req'd | Description                                                                  | Constraints |
| ----- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ---------------------------------------------------------------------------- | ----------- |
| `ref` | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | place.stream.media.source record providing the content for this video record |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.video",
  "defs": {
    "main": {
      "type": "record",
      "description": "Some audiovisual content.",
      "key": "tid",
      "record": {
        "required": ["title"],
        "type": "object",
        "properties": {
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140,
            "description": "Title of the video referenced by this record"
          },
          "source": {
            "type": "union",
            "refs": [
              "place.stream.media.defs#sourceTracks",
              "place.stream.media.defs#sourceClip"
            ],
            "description": "What is the source of this video?"
          },
          "description": {
            "type": "string",
            "maxLength": 50000,
            "maxGraphemes": 5000,
            "description": "Description of this video"
          },
          "duration": {
            "type": "integer",
            "description": "Duration of the video in milliseconds"
          },
          "descriptionFacets": {
            "type": "array",
            "description": "Annotations of text (mentions, URLs, etc)",
            "items": {
              "type": "ref",
              "ref": "place.stream.richtext.facet"
            }
          },
          "thumb": {
            "description": "Thumbnail image for the video.",
            "type": "blob",
            "accept": ["image/*"],
            "maxSize": 1000000
          },
          "connections": {
            "type": "array",
            "description": "Free-form list of atproto records related in some way to this video",
            "items": {
              "type": "union",
              "refs": ["#connection"]
            }
          },
          "contentPolicy": {
            "description": "copyright, distribution, and content warning data",
            "type": "ref",
            "ref": "place.stream.metadata.configuration"
          },
          "activity": {
            "type": "union",
            "description": "The game or activity in the video.",
            "refs": [
              "place.stream.defs#activityGame",
              "place.stream.defs#activityLabel"
            ]
          },
          "tags": {
            "type": "array",
            "description": "Freeform tags for this stream. Each tag must be alphanumeric (a-z, A-Z, 0-9) plus colon. Tags with colons indicate a specific tag group (e.g. 'lang:en' indicates the stream's primary language).",
            "maxLength": 10,
            "items": {
              "type": "string",
              "maxLength": 640,
              "maxGraphemes": 64
            }
          }
        }
      }
    },
    "connection": {
      "type": "object",
      "properties": {
        "ref": {
          "type": "ref",
          "ref": "com.atproto.repo.strongRef"
        }
      }
    },
    "videoSourceContent": {
      "type": "object",
      "properties": {
        "ref": {
          "description": "place.stream.media.source record providing the content for this video record",
          "type": "ref",
          "ref": "com.atproto.repo.strongRef"
        }
      }
    }
  }
}
```
