---
title: place.stream.branding.syncDomain
description: Reference for the place.stream.branding.syncDomain lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Pull a custom domain's brand record now instead of waiting for the firehose. Admins, or the domain's owner.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description | Constraints |
| ---------- | -------- | ----- | ----------- | ----------- |
| `hostname` | `string` | ✅    |             |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** [`place.stream.branding.defs#domainView`](/lex-reference/place-stream-branding-defs#domainview)

**Possible Errors:**

- `Unauthorized`
- `DomainNotFound`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.syncDomain",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Pull a custom domain's brand record now instead of waiting for the firehose. Admins, or the domain's owner.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["hostname"],
          "properties": {
            "hostname": {
              "type": "string"
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "ref",
          "ref": "place.stream.branding.defs#domainView"
        }
      },
      "errors": [
        {
          "name": "Unauthorized"
        },
        {
          "name": "DomainNotFound"
        }
      ]
    }
  }
}
```
