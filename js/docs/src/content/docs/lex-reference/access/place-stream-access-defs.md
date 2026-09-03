---
title: place.stream.access.defs
description: Reference for the place.stream.access.defs lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="role"></a>

### `role`

**Type:** `string`

A capability a node grants to an account. admin: manage branding and access control (implies every other role). viewer: use the frontend and playback when the node is private. streamer: ingest media to this node. syndicate: have this account's media carried from other nodes. vod: upload and record VODs.

**Constraints:**<br/>Known Values: `admin`, `viewer`, `streamer`, `syndicate`, `vod`

---

<a name="mode"></a>

### `mode`

**Type:** `string`

How a node decides a role. open: every account holds the role. allowlist: only accounts with a grant hold the role. off: nobody holds the role (admins are always exempt).

**Constraints:**<br/>Known Values: `open`, `allowlist`, `off`

---

<a name="rolemode"></a>

### `roleMode`

**Type:** `object`

**Properties:**

| Name   | Type             | Req'd | Description | Constraints |
| ------ | ---------------- | ----- | ----------- | ----------- |
| `role` | [`#role`](#role) | ✅    |             |             |
| `mode` | [`#mode`](#mode) | ✅    |             |             |

---

<a name="grantview"></a>

### `grantView`

**Type:** `object`

One account's grant of one role. Grants stored as place.stream.access.grant records in the node's access-control space carry a uri and cid; grants seeded from the node's environment (SP_ADMIN_DIDS, SP_ALLOWED_STREAMS, SP_SYNDICATE) have no uri and cannot be revoked from the API.

**Properties:**

| Name        | Type             | Req'd | Description                                                                                                                                                                                           | Constraints                             |
| ----------- | ---------------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `uri`       | `string`         | ❌    | at://{authority}/space/place.stream.access.control/self/{author}/place.stream.access.grant/{rkey} (A space URI; not validated as a classic at-uri because the space form is newer than that grammar.) |                                         |
| `cid`       | `string`         | ❌    |                                                                                                                                                                                                       | Format: `cid`                           |
| `subject`   | `string`         | ✅    |                                                                                                                                                                                                       | Format: `did`                           |
| `role`      | [`#role`](#role) | ✅    |                                                                                                                                                                                                       |                                         |
| `source`    | `string`         | ✅    | space: a record in the access-control space, editable via the API. environment: seeded from the node's configuration.                                                                                 | Known Values: `space`, `environment`    |
| `createdBy` | `string`         | ❌    | The admin that created the grant (the record's author).                                                                                                                                               | Format: `did`                           |
| `createdAt` | `string`         | ❌    |                                                                                                                                                                                                       | Format: `datetime`                      |
| `note`      | `string`         | ❌    |                                                                                                                                                                                                       | Max Length: 1000<br/>Max Graphemes: 100 |

---

<a name="policyview"></a>

### `policyView`

**Type:** `object`

**Properties:**

| Name    | Type                              | Req'd | Description                                                                                                                             | Constraints |
| ------- | --------------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `roles` | Array of [`#roleMode`](#rolemode) | ✅    | The effective mode of every role the node knows about, including environment overrides such as SP_WIDE_OPEN and SP_DISABLE_SYNDICATION. |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.access.defs",
  "defs": {
    "role": {
      "type": "string",
      "description": "A capability a node grants to an account. admin: manage branding and access control (implies every other role). viewer: use the frontend and playback when the node is private. streamer: ingest media to this node. syndicate: have this account's media carried from other nodes. vod: upload and record VODs.",
      "knownValues": ["admin", "viewer", "streamer", "syndicate", "vod"]
    },
    "mode": {
      "type": "string",
      "description": "How a node decides a role. open: every account holds the role. allowlist: only accounts with a grant hold the role. off: nobody holds the role (admins are always exempt).",
      "knownValues": ["open", "allowlist", "off"]
    },
    "roleMode": {
      "type": "object",
      "required": ["role", "mode"],
      "properties": {
        "role": {
          "type": "ref",
          "ref": "#role"
        },
        "mode": {
          "type": "ref",
          "ref": "#mode"
        }
      }
    },
    "grantView": {
      "type": "object",
      "description": "One account's grant of one role. Grants stored as place.stream.access.grant records in the node's access-control space carry a uri and cid; grants seeded from the node's environment (SP_ADMIN_DIDS, SP_ALLOWED_STREAMS, SP_SYNDICATE) have no uri and cannot be revoked from the API.",
      "required": ["subject", "role", "source"],
      "properties": {
        "uri": {
          "type": "string",
          "description": "at://{authority}/space/place.stream.access.control/self/{author}/place.stream.access.grant/{rkey} (A space URI; not validated as a classic at-uri because the space form is newer than that grammar.)"
        },
        "cid": {
          "type": "string",
          "format": "cid"
        },
        "subject": {
          "type": "string",
          "format": "did"
        },
        "role": {
          "type": "ref",
          "ref": "#role"
        },
        "source": {
          "type": "string",
          "knownValues": ["space", "environment"],
          "description": "space: a record in the access-control space, editable via the API. environment: seeded from the node's configuration."
        },
        "createdBy": {
          "type": "string",
          "format": "did",
          "description": "The admin that created the grant (the record's author)."
        },
        "createdAt": {
          "type": "string",
          "format": "datetime"
        },
        "note": {
          "type": "string",
          "maxLength": 1000,
          "maxGraphemes": 100
        }
      }
    },
    "policyView": {
      "type": "object",
      "required": ["roles"],
      "properties": {
        "roles": {
          "type": "array",
          "description": "The effective mode of every role the node knows about, including environment overrides such as SP_WIDE_OPEN and SP_DISABLE_SYNDICATION.",
          "items": {
            "type": "ref",
            "ref": "#roleMode"
          }
        }
      }
    }
  }
}
```
