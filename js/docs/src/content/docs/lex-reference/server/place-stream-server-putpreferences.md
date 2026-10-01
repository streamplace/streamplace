---
title: place.stream.server.putPreferences
description: Reference for the place.stream.server.putPreferences lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Update the authenticated user's private preferences on this node. Omitted fields keep their current values.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name              | Type      | Req'd | Description                                                                                                 | Constraints |
| ----------------- | --------- | ----- | ----------------------------------------------------------------------------------------------------------- | ----------- |
| `autoPublishVods` | `boolean` | ❌    | Whether the node publishes a VOD of each of the user's recorded livestreams as soon as the livestream ends. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name          | Type                                                                                          | Req'd | Description | Constraints |
| ------------- | --------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `preferences` | [`place.stream.server.defs#preferences`](/lex-reference/place-stream-server-defs#preferences) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.server.putPreferences",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Update the authenticated user's private preferences on this node. Omitted fields keep their current values.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": [],
          "properties": {
            "autoPublishVods": {
              "type": "boolean",
              "description": "Whether the node publishes a VOD of each of the user's recorded livestreams as soon as the livestream ends."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["preferences"],
          "properties": {
            "preferences": {
              "type": "ref",
              "ref": "place.stream.server.defs#preferences"
            }
          }
        }
      }
    }
  }
}
```
