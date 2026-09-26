---
title: place.stream.branding.putDomain
description: Reference for the place.stream.branding.putDomain lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Serve a custom domain from this node under its owner's brand record. Admins only. Point the domain's DNS at the node; the node then pulls the owner's place.stream.branding.brand record keyed by the hostname.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type     | Req'd | Description                                                | Constraints   |
| ---------- | -------- | ----- | ---------------------------------------------------------- | ------------- |
| `hostname` | `string` | ✅    | The custom domain, e.g. live.example.com.                  |               |
| `owner`    | `string` | ❌    | Account that owns the domain's brand. Default: the caller. | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** [`place.stream.branding.defs#domainView`](/lex-reference/place-stream-branding-defs#domainview)

**Possible Errors:**

- `Unauthorized`
- `InvalidHostname`

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.putDomain",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Serve a custom domain from this node under its owner's brand record. Admins only. Point the domain's DNS at the node; the node then pulls the owner's place.stream.branding.brand record keyed by the hostname.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["hostname"],
          "properties": {
            "hostname": {
              "type": "string",
              "description": "The custom domain, e.g. live.example.com."
            },
            "owner": {
              "type": "string",
              "format": "did",
              "description": "Account that owns the domain's brand. Default: the caller."
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
          "name": "InvalidHostname"
        }
      ]
    }
  }
}
```
