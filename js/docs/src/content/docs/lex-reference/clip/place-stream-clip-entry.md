---
title: place.stream.clip.entry
description: Reference for the place.stream.clip.entry lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A clip created from a livestream by a viewer. This record is the authoritative source for the clip's title and description; the referenced place.stream.video carries a copy. The clip record links the video content to its source livestream for provenance and discovery.

**Record Key:** `tid`

**Record Properties:**

| Name          | Type                                                                                                                                   | Req'd | Description                                                                    | Constraints                               |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------ | ----------------------------------------- |
| `video`       | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ✅    | The place.stream.video record that contains the clip's playable content.       |                                           |
| `livestream`  | `string`                                                                                                                               | ✅    | AT URI of the place.stream.livestream this clip was created from.              | Format: `at-uri`                          |
| `start`       | `integer`                                                                                                                              | ✅    | Start time of the clip in the original livestream, in milliseconds.            |                                           |
| `end`         | `integer`                                                                                                                              | ✅    | End time of the clip in the original livestream, in milliseconds.              |                                           |
| `title`       | `string`                                                                                                                               | ✅    | Title of the clip. Authoritative; copied to the referenced video record.       | Max Length: 1400<br/>Max Graphemes: 140   |
| `description` | `string`                                                                                                                               | ❌    | Description of the clip. Authoritative; copied to the referenced video record. | Max Length: 50000<br/>Max Graphemes: 5000 |
| `createdAt`   | `string`                                                                                                                               | ✅    | Timestamp when this clip record was created.                                   | Format: `datetime`                        |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.clip.entry",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "A clip created from a livestream by a viewer. This record is the authoritative source for the clip's title and description; the referenced place.stream.video carries a copy. The clip record links the video content to its source livestream for provenance and discovery.",
      "record": {
        "type": "object",
        "required": [
          "video",
          "livestream",
          "start",
          "end",
          "title",
          "createdAt"
        ],
        "properties": {
          "video": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The place.stream.video record that contains the clip's playable content."
          },
          "livestream": {
            "type": "string",
            "format": "at-uri",
            "description": "AT URI of the place.stream.livestream this clip was created from."
          },
          "start": {
            "type": "integer",
            "description": "Start time of the clip in the original livestream, in milliseconds."
          },
          "end": {
            "type": "integer",
            "description": "End time of the clip in the original livestream, in milliseconds."
          },
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140,
            "description": "Title of the clip. Authoritative; copied to the referenced video record."
          },
          "description": {
            "type": "string",
            "maxLength": 50000,
            "maxGraphemes": 5000,
            "description": "Description of the clip. Authoritative; copied to the referenced video record."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Timestamp when this clip record was created."
          }
        }
      }
    }
  }
}
```
