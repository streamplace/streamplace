---
title: place.stream.badge.getIssuedBadges
description: Reference for the place.stream.badge.getIssuedBadges lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Returns badges issued to the authenticated user, organized by display slot. Also indicates which badges are currently selected in the user's chat profile.

**Parameters:**

| Name       | Type     | Req'd | Description                                                      | Constraints   |
| ---------- | -------- | ----- | ---------------------------------------------------------------- | ------------- |
| `streamer` | `string` | ❌    | DID of the streamer channel to compute the server badge against. | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type                                                                                    | Req'd | Description                                                                                                          | Constraints |
| ---------- | --------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------------------------------------------------------- | ----------- |
| `server`   | [`place.stream.badge.defs#badgeView`](/lex-reference/place-stream-badge-defs#badgeview) | ❌    | Computed server badge (streamer, mod, or bot) for the given streamer context. Absent if the user has no server role. |             |
| `streamer` | [`place.stream.badge.defs#badgeSlot`](/lex-reference/place-stream-badge-defs#badgeslot) | ✅    | Streamer-issued badges (e.g. VIP) available to the user.                                                             |             |
| `user`     | [`place.stream.badge.defs#badgeSlot`](/lex-reference/place-stream-badge-defs#badgeslot) | ✅    | Globally-issued cosmetic badges (e.g. event) available to the user.                                                  |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.badge.getIssuedBadges",
  "defs": {
    "main": {
      "type": "query",
      "description": "Returns badges issued to the authenticated user, organized by display slot. Also indicates which badges are currently selected in the user's chat profile.",
      "parameters": {
        "type": "params",
        "properties": {
          "streamer": {
            "type": "string",
            "format": "did",
            "description": "DID of the streamer channel to compute the server badge against."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "user"],
          "properties": {
            "server": {
              "type": "ref",
              "ref": "place.stream.badge.defs#badgeView",
              "description": "Computed server badge (streamer, mod, or bot) for the given streamer context. Absent if the user has no server role."
            },
            "streamer": {
              "type": "ref",
              "ref": "place.stream.badge.defs#badgeSlot",
              "description": "Streamer-issued badges (e.g. VIP) available to the user."
            },
            "user": {
              "type": "ref",
              "ref": "place.stream.badge.defs#badgeSlot",
              "description": "Globally-issued cosmetic badges (e.g. event) available to the user."
            }
          }
        }
      }
    }
  }
}
```
