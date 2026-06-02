---
title: place.stream.beta.getStatus
description: Reference for the place.stream.beta.getStatus lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Report an account's access status for a named beta feature: whether it has been granted (via a trusted place.stream.beta.invite, or because this node isn't beta-gating the feature), has an outstanding place.stream.beta.request, or neither. Defaults to the authenticated caller when `did` is omitted.

**Parameters:**

| Name      | Type     | Req'd | Description                                                                | Constraints   |
| --------- | -------- | ----- | -------------------------------------------------------------------------- | ------------- |
| `feature` | `string` | ✅    | Identifier of the beta feature to check (e.g. "vod").                      |               |
| `did`     | `string` | ❌    | Account to check. Defaults to the authenticated caller's DID when omitted. | Format: `did` |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type     | Req'd | Description                                                                                                                         | Constraints                                  |
| --------- | -------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| `feature` | `string` | ✅    | The feature this status applies to (echoes the request).                                                                            |                                              |
| `did`     | `string` | ✅    | The account this status applies to.                                                                                                 | Format: `did`                                |
| `status`  | `string` | ✅    | granted: the account may use the feature. requested: no grant yet, but an access request is on file. none: no grant and no request. | Known Values: `granted`, `requested`, `none` |

**Possible Errors:**

- `DidRequired`: No did supplied and the request was not authenticated, so there is no account to check.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.beta.getStatus",
  "defs": {
    "main": {
      "type": "query",
      "description": "Report an account's access status for a named beta feature: whether it has been granted (via a trusted place.stream.beta.invite, or because this node isn't beta-gating the feature), has an outstanding place.stream.beta.request, or neither. Defaults to the authenticated caller when `did` is omitted.",
      "parameters": {
        "type": "params",
        "required": ["feature"],
        "properties": {
          "feature": {
            "type": "string",
            "description": "Identifier of the beta feature to check (e.g. \"vod\")."
          },
          "did": {
            "type": "string",
            "format": "did",
            "description": "Account to check. Defaults to the authenticated caller's DID when omitted."
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["feature", "did", "status"],
          "properties": {
            "feature": {
              "type": "string",
              "description": "The feature this status applies to (echoes the request)."
            },
            "did": {
              "type": "string",
              "format": "did",
              "description": "The account this status applies to."
            },
            "status": {
              "type": "string",
              "knownValues": ["granted", "requested", "none"],
              "description": "granted: the account may use the feature. requested: no grant yet, but an access request is on file. none: no grant and no request."
            }
          }
        }
      },
      "errors": [
        {
          "name": "DidRequired",
          "description": "No did supplied and the request was not authenticated, so there is no account to check."
        }
      ]
    }
  }
}
```
