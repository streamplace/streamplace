---
title: place.stream.bio.defs
description: Reference for the place.stream.bio.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="social"></a>

### `social`

**Type:** `object`

A link to a profile or account on another platform.

**Properties:**

| Name       | Type     | Req'd | Description                                                                                                                 | Constraints                                                                                                                                                              |
| ---------- | -------- | ----- | --------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `platform` | `string` | ✅    | Identifier for the platform. Clients should render a recognizable icon for known values and fall back gracefully otherwise. | Max Length: 64<br/>Known Values: `bluesky`, `twitter`, `youtube`, `twitch`, `kick`, `discord`, `instagram`, `tiktok`, `github`, `cashapp`, `ko-fi`, `patreon`, `website` |
| `handle`   | `string` | ❌    | Display handle for this account, e.g. '@alice'.                                                                             | Max Length: 256<br/>Max Graphemes: 64                                                                                                                                    |
| `url`      | `string` | ✅    | URL to the account.                                                                                                         | Format: `uri`                                                                                                                                                            |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.defs",
  "defs": {
    "social": {
      "type": "object",
      "description": "A link to a profile or account on another platform.",
      "required": ["platform", "url"],
      "properties": {
        "platform": {
          "type": "string",
          "description": "Identifier for the platform. Clients should render a recognizable icon for known values and fall back gracefully otherwise.",
          "knownValues": [
            "bluesky",
            "twitter",
            "youtube",
            "twitch",
            "kick",
            "discord",
            "instagram",
            "tiktok",
            "github",
            "cashapp",
            "ko-fi",
            "patreon",
            "website"
          ],
          "maxLength": 64
        },
        "handle": {
          "type": "string",
          "description": "Display handle for this account, e.g. '@alice'.",
          "maxLength": 256,
          "maxGraphemes": 64
        },
        "url": {
          "type": "string",
          "format": "uri",
          "description": "URL to the account."
        }
      }
    }
  }
}
```
