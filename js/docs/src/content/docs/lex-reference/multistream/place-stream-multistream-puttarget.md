---
title: place.stream.multistream.putTarget
description: Reference for the place.stream.multistream.putTarget lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Update an existing target for rebroadcasting a Streamplace stream.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name                | Type                                                                                | Req'd | Description     | Constraints                              |
| ------------------- | ----------------------------------------------------------------------------------- | ----- | --------------- | ---------------------------------------- |
| `multistreamTarget` | [`place.stream.multistream.target`](/lex-reference/place-stream-multistream-target) | ✅    |                 |                                          |
| `rkey`              | `string`                                                                            | ❌    | The Record Key. | Format: `record-key`<br/>Max Length: 512 |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** [`place.stream.multistream.defs#targetView`](/lex-reference/place-stream-multistream-defs#targetview)

**Possible Errors:**

- `InvalidTargetUrl`: The provided target URL is invalid or unreachable.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.multistream.putTarget",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Update an existing target for rebroadcasting a Streamplace stream.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["multistreamTarget"],
          "properties": {
            "multistreamTarget": {
              "type": "ref",
              "ref": "place.stream.multistream.target"
            },
            "rkey": {
              "type": "string",
              "format": "record-key",
              "description": "The Record Key.",
              "maxLength": 512
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "ref",
          "ref": "place.stream.multistream.defs#targetView"
        }
      },
      "errors": [
        {
          "name": "InvalidTargetUrl",
          "description": "The provided target URL is invalid or unreachable."
        }
      ]
    }
  }
}
```
