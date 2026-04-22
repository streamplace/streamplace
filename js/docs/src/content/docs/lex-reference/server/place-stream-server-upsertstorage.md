---
title: place.stream.server.upsertStorage
description: Reference for the place.stream.server.upsertStorage lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Create or update S3 storage configuration for backups.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name       | Type      | Req'd | Description                                                                | Constraints |
| ---------- | --------- | ----- | -------------------------------------------------------------------------- | ----------- |
| `url`      | `string`  | ❌    | S3 storage URL in format: s3+https://ACCESS_KEY:SECRET_KEY@endpoint/bucket |             |
| `isActive` | `boolean` | ❌    | Whether backup storage is currently active.                                |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                                  | Req'd | Description | Constraints |
| --------- | ------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `storage` | [`place.stream.server.defs#storage`](/lex-reference/place-stream-server-defs#storage) | ✅    |             |             |

**Possible Errors:**

- `InvalidUrl`: The provided S3 URL is invalid or malformed.
- `ConnectionFailed`: Could not connect to the S3 endpoint with the provided credentials.
- `MaskedCredentialsModified`: Cannot modify URL while keeping masked credentials. Provide full credentials or omit URL to keep existing configuration.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.upsertStorage",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Create or update S3 storage configuration for backups.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": [],
          "properties": {
            "url": {
              "type": "string",
              "description": "S3 storage URL in format: s3+https://ACCESS_KEY:SECRET_KEY@endpoint/bucket"
            },
            "isActive": {
              "type": "boolean",
              "description": "Whether backup storage is currently active."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["storage"],
          "properties": {
            "storage": {
              "type": "ref",
              "ref": "place.stream.server.defs#storage"
            }
          }
        }
      },
      "errors": [
        {
          "name": "InvalidUrl",
          "description": "The provided S3 URL is invalid or malformed."
        },
        {
          "name": "ConnectionFailed",
          "description": "Could not connect to the S3 endpoint with the provided credentials."
        },
        {
          "name": "MaskedCredentialsModified",
          "description": "Cannot modify URL while keeping masked credentials. Provide full credentials or omit URL to keep existing configuration."
        }
      ]
    }
  }
}
```
