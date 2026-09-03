---
title: place.stream.access.getStatus
description: Reference for the place.stream.access.getStatus lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Report the caller's roles on this node and the node's access policy. Works unauthenticated (roles then reflect what an anonymous visitor holds). This is the one place.stream method a node always answers, even to accounts locked out by a private viewer policy, so clients can render the right wall.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type                                                                                        | Req'd | Description                                                                                                                                                                                  | Constraints   |
| -------- | ------------------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| `did`    | `string`                                                                                    | ❌    | The authenticated caller, when there is one.                                                                                                                                                 | Format: `did` |
| `roles`  | Array of [`place.stream.access.defs#role`](/lex-reference/place-stream-access-defs#role)    | ✅    | Every role the caller effectively holds.                                                                                                                                                     |               |
| `policy` | [`place.stream.access.defs#policyView`](/lex-reference/place-stream-access-defs#policyview) | ✅    |                                                                                                                                                                                              |               |
| `space`  | `string`                                                                                    | ✅    | The node's access-control space: at://{authority}/space/place.stream.access.control/self (A space URI; not validated as a classic at-uri because the space form is newer than that grammar.) |               |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.getStatus",
  "defs": {
    "main": {
      "type": "query",
      "description": "Report the caller's roles on this node and the node's access policy. Works unauthenticated (roles then reflect what an anonymous visitor holds). This is the one place.stream method a node always answers, even to accounts locked out by a private viewer policy, so clients can render the right wall.",
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["roles", "policy", "space"],
          "properties": {
            "did": {
              "type": "string",
              "format": "did",
              "description": "The authenticated caller, when there is one."
            },
            "roles": {
              "type": "array",
              "description": "Every role the caller effectively holds.",
              "items": {
                "type": "ref",
                "ref": "place.stream.access.defs#role"
              }
            },
            "policy": {
              "type": "ref",
              "ref": "place.stream.access.defs#policyView"
            },
            "space": {
              "type": "string",
              "description": "The node's access-control space: at://{authority}/space/place.stream.access.control/self (A space URI; not validated as a classic at-uri because the space form is newer than that grammar.)"
            }
          }
        }
      }
    }
  }
}
```
