---
title: place.stream.metadata.contentRights
description: Reference for the place.stream.metadata.contentRights lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

Content rights and attribution information.

**Properties:**

| Name              | Type      | Req'd | Description                      | Constraints                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| ----------------- | --------- | ----- | -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `creator`         | `string`  | ❌    | Name of the creator of the work. |                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `copyrightNotice` | `string`  | ❌    | Copyright notice for the work.   |                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `copyrightYear`   | `integer` | ❌    | Year of creation or publication. |                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `license`         | `string`  | ❌    | License URL or identifier.       | Known Values: `place.stream.metadata.contentRights#all-rights-reserved`, `place.stream.metadata.contentRights#cc0_1__0`, `place.stream.metadata.contentRights#cc-by_4__0`, `place.stream.metadata.contentRights#cc-by-sa_4__0`, `place.stream.metadata.contentRights#cc-by-nc_4__0`, `place.stream.metadata.contentRights#cc-by-nc-sa_4__0`, `place.stream.metadata.contentRights#cc-by-nd_4__0`, `place.stream.metadata.contentRights#cc-by-nc-nd_4__0` |
| `creditLine`      | `string`  | ❌    | Credit line for the work.        |                                                                                                                                                                                                                                                                                                                                                                                                                                                          |

---

<a name="allrightsreserved"></a>

### `all-rights-reserved`

**Type:** `token`

All rights reserved to the creator — others cannot use, modify, or share without
explicit authorization.

---

<a name="cc010"></a>

### `cc0_1__0`

**Type:** `token`

Public domain dedication. You waive all copyright and related rights where
possible. Others may copy, modify, distribute, or perform your work for any
purpose without attribution.

---

<a name="ccby40"></a>

### `cc-by_4__0`

**Type:** `token`

Attribution required. Others may copy, distribute, remix, and build upon your
work, even commercially, if they credit you.

---

<a name="ccbysa40"></a>

### `cc-by-sa_4__0`

**Type:** `token`

Attribution + share-alike. Others may adapt and build upon your work, even
commercially, if they credit you and license their new creations under identical
terms.

---

<a name="ccbync40"></a>

### `cc-by-nc_4__0`

**Type:** `token`

Attribution + non-commercial. Others may adapt and build upon your work for
non-commercial purposes only, and must credit you.

---

<a name="ccbyncsa40"></a>

### `cc-by-nc-sa_4__0`

**Type:** `token`

Attribution + non-commercial + share-alike. Others may adapt and build upon your
work for non-commercial purposes only, must credit you, and must license their
new creations under identical terms.

---

<a name="ccbynd40"></a>

### `cc-by-nd_4__0`

**Type:** `token`

Attribution + no derivatives. Others may reuse your work, even commercially, but
it must remain unchanged and you must be credited.

---

<a name="ccbyncnd40"></a>

### `cc-by-nc-nd_4__0`

**Type:** `token`

Attribution + non-commercial + no derivatives. Others may download and share
your work with credit, but cannot change it or use it commercially.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.metadata.contentRights",
  "defs": {
    "main": {
      "type": "object",
      "description": "Content rights and attribution information.",
      "properties": {
        "creator": {
          "type": "string",
          "description": "Name of the creator of the work."
        },
        "copyrightNotice": {
          "type": "string",
          "description": "Copyright notice for the work."
        },
        "copyrightYear": {
          "type": "integer",
          "description": "Year of creation or publication."
        },
        "license": {
          "type": "string",
          "description": "License URL or identifier.",
          "knownValues": [
            "place.stream.metadata.contentRights#all-rights-reserved",
            "place.stream.metadata.contentRights#cc0_1__0",
            "place.stream.metadata.contentRights#cc-by_4__0",
            "place.stream.metadata.contentRights#cc-by-sa_4__0",
            "place.stream.metadata.contentRights#cc-by-nc_4__0",
            "place.stream.metadata.contentRights#cc-by-nc-sa_4__0",
            "place.stream.metadata.contentRights#cc-by-nd_4__0",
            "place.stream.metadata.contentRights#cc-by-nc-nd_4__0"
          ]
        },
        "creditLine": {
          "type": "string",
          "description": "Credit line for the work."
        }
      }
    },
    "all-rights-reserved": {
      "type": "token",
      "description": "All rights reserved to the creator — others cannot use, modify, or share without explicit authorization."
    },
    "cc0_1__0": {
      "type": "token",
      "description": "Public domain dedication. You waive all copyright and related rights where possible. Others may copy, modify, distribute, or perform your work for any purpose without attribution."
    },
    "cc-by_4__0": {
      "type": "token",
      "description": "Attribution required. Others may copy, distribute, remix, and build upon your work, even commercially, if they credit you."
    },
    "cc-by-sa_4__0": {
      "type": "token",
      "description": "Attribution + share-alike. Others may adapt and build upon your work, even commercially, if they credit you and license their new creations under identical terms."
    },
    "cc-by-nc_4__0": {
      "type": "token",
      "description": "Attribution + non-commercial. Others may adapt and build upon your work for non-commercial purposes only, and must credit you."
    },
    "cc-by-nc-sa_4__0": {
      "type": "token",
      "description": "Attribution + non-commercial + share-alike. Others may adapt and build upon your work for non-commercial purposes only, must credit you, and must license their new creations under identical terms."
    },
    "cc-by-nd_4__0": {
      "type": "token",
      "description": "Attribution + no derivatives. Others may reuse your work, even commercially, but it must remain unchanged and you must be credited."
    },
    "cc-by-nc-nd_4__0": {
      "type": "token",
      "description": "Attribution + non-commercial + no derivatives. Others may download and share your work with credit, but cannot change it or use it commercially."
    }
  }
}
```
