---
title: place.stream.livestream
description: Reference for the place.stream.livestream lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Record announcing a livestream is happening

**Record Key:** `tid`

**Record Properties:**

| Name                   | Type                                                                                                                                                                                                            | Req'd | Description                                                                                                                                                                                       | Constraints                                   |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `title`                | `string`                                                                                                                                                                                                        | ✅    | The title of the livestream, as it will be announced to followers.                                                                                                                                | Max Length: 1400<br/>Max Graphemes: 140       |
| `url`                  | `string`                                                                                                                                                                                                        | ❌    | The URL where this stream can be found. This is primarily a hint for other Streamplace nodes to locate and replicate the stream.                                                                  | Format: `uri`                                 |
| `createdAt`            | `string`                                                                                                                                                                                                        | ✅    | Client-declared timestamp when this livestream started.                                                                                                                                           | Format: `datetime`                            |
| `lastSeenAt`           | `string`                                                                                                                                                                                                        | ❌    | Client-declared timestamp when this livestream was last seen by the Streamplace station.                                                                                                          | Format: `datetime`                            |
| `endedAt`              | `string`                                                                                                                                                                                                        | ❌    | Client-declared timestamp when this livestream ended. Ended livestreams are not supposed to start up again.                                                                                       | Format: `datetime`                            |
| `idleTimeoutSeconds`   | `integer`                                                                                                                                                                                                       | ❌    | Time in seconds after which this livestream should be automatically ended if idle. Zero means no timeout.                                                                                         |                                               |
| `post`                 | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined)                                                                          | ❌    | The post that announced this livestream.                                                                                                                                                          |                                               |
| `agent`                | `string`                                                                                                                                                                                                        | ❌    | The source of the livestream, if available, in a User Agent format: `<product> / <product-version> <comment>` e.g. Streamplace/0.7.5 iOS                                                          |                                               |
| `canonicalUrl`         | `string`                                                                                                                                                                                                        | ❌    | The primary URL where this livestream can be viewed, if available.                                                                                                                                | Format: `uri`                                 |
| `thumb`                | `blob`                                                                                                                                                                                                          | ❌    |                                                                                                                                                                                                   | Accept: `image/*`<br/>Max Size: 1000000 bytes |
| `notificationSettings` | [`place.stream.livestream#notificationSettings`](/lex-reference/place-stream-livestream#notificationsettings)                                                                                                   | ❌    |                                                                                                                                                                                                   |                                               |
| `activity`             | Union of:<br/>&nbsp;&nbsp;[`place.stream.defs#activityGame`](/lex-reference/place-stream-defs#activitygame)<br/>&nbsp;&nbsp;[`place.stream.defs#activityLabel`](/lex-reference/place-stream-defs#activitylabel) | ❌    | The game or activity being streamed.                                                                                                                                                              |                                               |
| `tags`                 | Array of `string`                                                                                                                                                                                               | ❌    | Freeform tags for this stream. Each tag must be alphanumeric (a-z, A-Z, 0-9) plus colon. Tags with colons indicate a specific tag group (e.g. 'lang:en' indicates the stream's primary language). | Max Items: 10                                 |

---

<a name="notificationsettings"></a>

### `notificationSettings`

**Type:** `object`

**Properties:**

| Name               | Type      | Req'd | Description                                                              | Constraints |
| ------------------ | --------- | ----- | ------------------------------------------------------------------------ | ----------- |
| `pushNotification` | `boolean` | ❌    | Whether this livestream should trigger a push notification to followers. |             |

---

<a name="livestreamview"></a>

### `livestreamView`

**Type:** `object`

**Properties:**

| Name          | Type                                                                                                                                             | Req'd | Description                                                                                              | Constraints        |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | -------------------------------------------------------------------------------------------------------- | ------------------ |
| `uri`         | `string`                                                                                                                                         | ✅    |                                                                                                          | Format: `at-uri`   |
| `cid`         | `string`                                                                                                                                         | ✅    |                                                                                                          | Format: `cid`      |
| `author`      | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    |                                                                                                          |                    |
| `record`      | `unknown`                                                                                                                                        | ✅    |                                                                                                          |                    |
| `indexedAt`   | `string`                                                                                                                                         | ✅    |                                                                                                          | Format: `datetime` |
| `viewerCount` | [`#viewerCount`](#viewercount)                                                                                                                   | ❌    | The number of viewers watching this livestream. Use when you can't reasonably use #viewerCount directly. |                    |

---

<a name="viewercount"></a>

### `viewerCount`

**Type:** `object`

**Properties:**

| Name    | Type      | Req'd | Description | Constraints |
| ------- | --------- | ----- | ----------- | ----------- |
| `count` | `integer` | ✅    |             |             |

---

<a name="teleportarrival"></a>

### `teleportArrival`

**Type:** `object`

**Properties:**

| Name          | Type                                                                                                                                             | Req'd | Description                                        | Constraints        |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----- | -------------------------------------------------- | ------------------ |
| `teleportUri` | `string`                                                                                                                                         | ✅    | The URI of the teleport record                     | Format: `at-uri`   |
| `source`      | [`app.bsky.actor.defs#profileViewBasic`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/actor/defs.json#profileViewBasic) | ✅    | The streamer who is teleporting their viewers here |                    |
| `chatProfile` | [`place.stream.chat.profile`](/lex-reference/place-stream-chat-profile)                                                                          | ❌    | The chat profile of the source streamer            |                    |
| `viewerCount` | `integer`                                                                                                                                        | ✅    | How many viewers are arriving from this teleport   |                    |
| `startsAt`    | `string`                                                                                                                                         | ✅    | When this teleport started                         | Format: `datetime` |

---

<a name="teleportcanceled"></a>

### `teleportCanceled`

**Type:** `object`

**Properties:**

| Name          | Type     | Req'd | Description                                      | Constraints                          |
| ------------- | -------- | ----- | ------------------------------------------------ | ------------------------------------ |
| `teleportUri` | `string` | ✅    | The URI of the teleport record that was canceled | Format: `at-uri`                     |
| `reason`      | `string` | ✅    | Why this teleport was canceled                   | Enum: `deleted`, `denied`, `expired` |

---

<a name="streamplaceanything"></a>

### `streamplaceAnything`

**Type:** `object`

**Properties:**

| Name         | Type                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Req'd | Description | Constraints |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `livestream` | Union of:<br/>&nbsp;&nbsp;[`#livestreamView`](#livestreamview)<br/>&nbsp;&nbsp;[`#viewerCount`](#viewercount)<br/>&nbsp;&nbsp;[`#teleportArrival`](#teleportarrival)<br/>&nbsp;&nbsp;[`#teleportCanceled`](#teleportcanceled)<br/>&nbsp;&nbsp;[`place.stream.defs#blockView`](/lex-reference/place-stream-defs#blockview)<br/>&nbsp;&nbsp;[`place.stream.defs#renditions`](/lex-reference/place-stream-defs#renditions)<br/>&nbsp;&nbsp;[`place.stream.defs#rendition`](/lex-reference/place-stream-defs#rendition)<br/>&nbsp;&nbsp;[`place.stream.chat.defs#messageView`](/lex-reference/place-stream-chat-defs#messageview)<br/>&nbsp;&nbsp;[`place.stream.chat.defs#pinnedRecordView`](/lex-reference/place-stream-chat-defs#pinnedrecordview) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.livestream",
  "defs": {
    "main": {
      "type": "record",
      "description": "Record announcing a livestream is happening",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["title", "createdAt"],
        "properties": {
          "title": {
            "type": "string",
            "maxLength": 1400,
            "maxGraphemes": 140,
            "description": "The title of the livestream, as it will be announced to followers."
          },
          "url": {
            "type": "string",
            "format": "uri",
            "description": "The URL where this stream can be found. This is primarily a hint for other Streamplace nodes to locate and replicate the stream."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this livestream started."
          },
          "lastSeenAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this livestream was last seen by the Streamplace station."
          },
          "endedAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this livestream ended. Ended livestreams are not supposed to start up again."
          },
          "idleTimeoutSeconds": {
            "type": "integer",
            "description": "Time in seconds after which this livestream should be automatically ended if idle. Zero means no timeout."
          },
          "post": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The post that announced this livestream."
          },
          "agent": {
            "type": "string",
            "description": "The source of the livestream, if available, in a User Agent format: `<product> / <product-version> <comment>` e.g. Streamplace/0.7.5 iOS"
          },
          "canonicalUrl": {
            "type": "string",
            "format": "uri",
            "description": "The primary URL where this livestream can be viewed, if available."
          },
          "thumb": {
            "type": "blob",
            "accept": ["image/*"],
            "maxSize": 1000000
          },
          "notificationSettings": {
            "type": "ref",
            "ref": "place.stream.livestream#notificationSettings"
          },
          "activity": {
            "type": "union",
            "description": "The game or activity being streamed.",
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
    "notificationSettings": {
      "type": "object",
      "required": [],
      "properties": {
        "pushNotification": {
          "type": "boolean",
          "description": "Whether this livestream should trigger a push notification to followers."
        }
      }
    },
    "livestreamView": {
      "type": "object",
      "required": ["uri", "cid", "author", "record", "indexedAt"],
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
        },
        "indexedAt": {
          "type": "string",
          "format": "datetime"
        },
        "viewerCount": {
          "type": "ref",
          "description": "The number of viewers watching this livestream. Use when you can't reasonably use #viewerCount directly.",
          "ref": "#viewerCount"
        }
      }
    },
    "viewerCount": {
      "type": "object",
      "required": ["count"],
      "properties": {
        "count": {
          "type": "integer"
        }
      }
    },
    "teleportArrival": {
      "type": "object",
      "required": ["teleportUri", "source", "viewerCount", "startsAt"],
      "properties": {
        "teleportUri": {
          "type": "string",
          "format": "at-uri",
          "description": "The URI of the teleport record"
        },
        "source": {
          "type": "ref",
          "ref": "app.bsky.actor.defs#profileViewBasic",
          "description": "The streamer who is teleporting their viewers here"
        },
        "chatProfile": {
          "type": "ref",
          "ref": "place.stream.chat.profile",
          "description": "The chat profile of the source streamer"
        },
        "viewerCount": {
          "type": "integer",
          "description": "How many viewers are arriving from this teleport"
        },
        "startsAt": {
          "type": "string",
          "format": "datetime",
          "description": "When this teleport started"
        }
      }
    },
    "teleportCanceled": {
      "type": "object",
      "required": ["teleportUri", "reason"],
      "properties": {
        "teleportUri": {
          "type": "string",
          "format": "at-uri",
          "description": "The URI of the teleport record that was canceled"
        },
        "reason": {
          "type": "string",
          "enum": ["deleted", "denied", "expired"],
          "description": "Why this teleport was canceled"
        }
      }
    },
    "streamplaceAnything": {
      "type": "object",
      "required": ["livestream"],
      "properties": {
        "livestream": {
          "type": "union",
          "refs": [
            "#livestreamView",
            "#viewerCount",
            "#teleportArrival",
            "#teleportCanceled",
            "place.stream.defs#blockView",
            "place.stream.defs#renditions",
            "place.stream.defs#rendition",
            "place.stream.chat.defs#messageView",
            "place.stream.chat.defs#pinnedRecordView"
          ]
        }
      }
    }
  }
}
```
