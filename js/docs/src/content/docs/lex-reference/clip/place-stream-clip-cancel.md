---
title: place.stream.clip.cancel
description: Reference for the place.stream.clip.cancel lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Cancel an unpublished clip draft: deletes the ephemeral muxed file immediately instead of waiting for the 10-minute TTL sweep, and removes the draft row. Fails if the draft was already published.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name     | Type     | Req'd | Description                                       | Constraints |
| -------- | -------- | ----- | ------------------------------------------------- | ----------- |
| `clipId` | `string` | ✅    | The clip ID returned by place.stream.clip.create. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name        | Type      | Req'd | Description                                       | Constraints |
| ----------- | --------- | ----- | ------------------------------------------------- | ----------- |
| `cancelled` | `boolean` | ✅    | True when the draft's ephemeral file was deleted. |             |

**Possible Errors:**

- `Unauthorized`: The request lacks valid authentication credentials.
- `DraftNotFound`: No unpublished draft with the given ID belongs to the authenticated user.
- `AlreadyPublished`: The draft has already been published and cannot be cancelled.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.clip.cancel",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Cancel an unpublished clip draft: deletes the ephemeral muxed file immediately instead of waiting for the 10-minute TTL sweep, and removes the draft row. Fails if the draft was already published.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["clipId"],
          "properties": {
            "clipId": {
              "type": "string",
              "description": "The clip ID returned by place.stream.clip.create."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["cancelled"],
          "properties": {
            "cancelled": {
              "type": "boolean",
              "description": "True when the draft's ephemeral file was deleted."
            }
          }
        }
      },
      "errors": [
        {
          "name": "Unauthorized",
          "description": "The request lacks valid authentication credentials."
        },
        {
          "name": "DraftNotFound",
          "description": "No unpublished draft with the given ID belongs to the authenticated user."
        },
        {
          "name": "AlreadyPublished",
          "description": "The draft has already been published and cannot be cancelled."
        }
      ]
    }
  }
}
```
