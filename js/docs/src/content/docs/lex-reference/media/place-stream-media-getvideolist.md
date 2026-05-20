---
title: place.stream.media.getVideoList
description: Reference for the place.stream.media.getVideoList lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

List videos for a given repo DID, newest first. Returns hydrated video views with author info and view counts.

**Parameters:**

| Name     | Type      | Req'd | Description                                 | Constraints                           |
| -------- | --------- | ----- | ------------------------------------------- | ------------------------------------- |
| `repo`   | `string`  | ✅    | DID of the repo whose videos to list.       | Format: `did`                         |
| `limit`  | `integer` | ❌    | Maximum number of videos to return.         | Min: 1<br/>Max: 100<br/>Default: `25` |
| `cursor` | `string`  | ❌    | Pagination cursor from a previous response. |                                       |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                                     | Req'd | Description                                  | Constraints |
| -------- | -------------------------------------------------------------------------------------------------------- | ----- | -------------------------------------------- | ----------- |
| `videos` | Array of [`place.stream.media.getVideo#videoView`](/lex-reference/place-stream-media-getvideo#videoview) | ✅    |                                              |             |
| `cursor` | `string`                                                                                                 | ❌    | Pagination cursor for the next page, if any. |             |

**Possible Errors:**

- `RepoNotFound`: No repo indexed at the supplied DID.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.media.getVideoList",
  "defs": {
    "main": {
      "type": "query",
      "description": "List videos for a given repo DID, newest first. Returns hydrated video views with author info and view counts.",
      "parameters": {
        "type": "params",
        "required": ["repo"],
        "properties": {
          "repo": {
            "type": "string",
            "format": "did",
            "description": "DID of the repo whose videos to list."
          },
          "limit": {
            "type": "integer",
            "minimum": 1,
            "maximum": 100,
            "default": 25,
            "description": "Maximum number of videos to return."
          },
          "cursor": {
            "type": "string",
            "description": "Pagination cursor from a previous response."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["videos"],
          "properties": {
            "videos": {
              "type": "array",
              "items": {
                "type": "ref",
                "ref": "place.stream.media.getVideo#videoView"
              }
            },
            "cursor": {
              "type": "string",
              "description": "Pagination cursor for the next page, if any."
            }
          }
        }
      },
      "errors": [
        {
          "name": "RepoNotFound",
          "description": "No repo indexed at the supplied DID."
        }
      ]
    }
  }
}
```
