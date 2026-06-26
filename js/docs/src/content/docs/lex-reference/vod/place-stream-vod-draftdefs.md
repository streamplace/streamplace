---
title: place.stream.vod.draftDefs
description: Reference for the place.stream.vod.draftDefs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="draftview"></a>

### `draftView`

**Type:** `object`

A draft VOD with its ats:// URI and CID, for list/get responses.

**Properties:**

| Name     | Type                                                                        | Req'd | Description                                                    | Constraints |
| -------- | --------------------------------------------------------------------------- | ----- | -------------------------------------------------------------- | ----------- |
| `uri`    | `string`                                                                    | ✅    | The ats:// URI of the draft record.                            |             |
| `cid`    | `string`                                                                    | ✅    | CID (sha256 of CBOR record bytes, base32) of the draft record. |             |
| `record` | [`place.stream.vod.draftVideo`](/lex-reference/place-stream-vod-draftvideo) | ✅    |                                                                |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.vod.draftDefs",
  "defs": {
    "draftView": {
      "type": "object",
      "description": "A draft VOD with its ats:// URI and CID, for list/get responses.",
      "required": ["uri", "cid", "record"],
      "properties": {
        "uri": {
          "type": "string",
          "description": "The ats:// URI of the draft record."
        },
        "cid": {
          "type": "string",
          "description": "CID (sha256 of CBOR record bytes, base32) of the draft record."
        },
        "record": {
          "type": "ref",
          "ref": "place.stream.vod.draftVideo"
        }
      }
    }
  }
}
```
