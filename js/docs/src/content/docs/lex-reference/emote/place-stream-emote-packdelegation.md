---
title: place.stream.emote.packDelegation
description: Reference for the place.stream.emote.packDelegation lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

Grants a specific user global permission to use emotes from one of the author's packs.

**Record Key:** `tid`

**Record Properties:**

| Name        | Type                                                                                                                                            | Req'd | Description                                                                                 | Constraints        |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------- | ------------------ |
| `did`       | `string`                                                                                                                                        | ✅    | The DID of the user being granted access to use these emotes.                               | Format: `did`      |
| `pack`      | [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined)          | ✅    | The pack the emotes come from. Must be owned by the record author.                          |                    |
| `emotes`    | Array of [`com.atproto.repo.strongRef`](https://github.com/bluesky-social/atproto/tree/main/lexicons/com/atproto/repo/strongref.json#undefined) | ❌    | Specific emotes from the pack to delegate. If absent, all emotes in the pack are delegated. |                    |
| `createdAt` | `string`                                                                                                                                        | ✅    | Client-declared timestamp when this delegation was created.                                 | Format: `datetime` |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.emote.packDelegation",
  "defs": {
    "main": {
      "type": "record",
      "description": "Grants a specific user global permission to use emotes from one of the author's packs.",
      "key": "tid",
      "record": {
        "type": "object",
        "required": ["did", "pack", "createdAt"],
        "properties": {
          "did": {
            "type": "string",
            "format": "did",
            "description": "The DID of the user being granted access to use these emotes."
          },
          "pack": {
            "type": "ref",
            "ref": "com.atproto.repo.strongRef",
            "description": "The pack the emotes come from. Must be owned by the record author."
          },
          "emotes": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "com.atproto.repo.strongRef"
            },
            "description": "Specific emotes from the pack to delegate. If absent, all emotes in the pack are delegated."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "Client-declared timestamp when this delegation was created."
          }
        }
      }
    }
  }
}
```
