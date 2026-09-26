---
title: place.stream.branding.defs
description: Reference for the place.stream.branding.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="domainview"></a>

### `domainView`

**Type:** `object`

A custom domain: a hostname this node serves under its owner's brand (their place.stream.branding.brand record keyed by the hostname) instead of the node's own.

**Properties:**

| Name        | Type     | Req'd | Description                                                                              | Constraints        |
| ----------- | -------- | ----- | ---------------------------------------------------------------------------------------- | ------------------ |
| `hostname`  | `string` | ✅    | The custom domain, lowercase, without scheme or port.                                    |                    |
| `owner`     | `string` | ✅    | Account whose brand record styles the domain, and who may change it.                     | Format: `did`      |
| `brand`     | `string` | ✅    | The broadcaster ID the domain's branding is read and written under (did:web:<hostname>). | Format: `did`      |
| `record`    | `string` | ❌    | The brand record the domain follows.                                                     | Format: `at-uri`   |
| `recordCid` | `string` | ❌    | Version of the record the node last applied.                                             | Format: `cid`      |
| `syncedAt`  | `string` | ❌    | When the node last pulled the record.                                                    | Format: `datetime` |
| `syncError` | `string` | ❌    | Why the last pull failed, if it did.                                                     |                    |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.defs",
  "defs": {
    "domainView": {
      "type": "object",
      "description": "A custom domain: a hostname this node serves under its owner's brand (their place.stream.branding.brand record keyed by the hostname) instead of the node's own.",
      "required": ["hostname", "owner", "brand"],
      "properties": {
        "hostname": {
          "type": "string",
          "description": "The custom domain, lowercase, without scheme or port."
        },
        "owner": {
          "type": "string",
          "format": "did",
          "description": "Account whose brand record styles the domain, and who may change it."
        },
        "brand": {
          "type": "string",
          "format": "did",
          "description": "The broadcaster ID the domain's branding is read and written under (did:web:<hostname>)."
        },
        "record": {
          "type": "string",
          "format": "at-uri",
          "description": "The brand record the domain follows."
        },
        "recordCid": {
          "type": "string",
          "format": "cid",
          "description": "Version of the record the node last applied."
        },
        "syncedAt": {
          "type": "string",
          "format": "datetime",
          "description": "When the node last pulled the record."
        },
        "syncError": {
          "type": "string",
          "description": "Why the last pull failed, if it did."
        }
      }
    }
  }
}
```
