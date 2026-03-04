---
title: place.stream.ingest.getIngestUrls
description: Reference for the place.stream.ingest.getIngestUrls lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `query`

Get ingest URLs for a Streamplace station.

**Parameters:** _(None defined)_

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name      | Type                                                                                                                   | Req'd | Description | Constraints |
| --------- | ---------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `ingests` | Array of Union of:<br/>&nbsp;&nbsp;[`place.stream.ingest.defs#ingest`](/lex-reference/place-stream-ingest-defs#ingest) | ✅    |             |             |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.ingest.getIngestUrls",
  "defs": {
    "main": {
      "type": "query",
      "description": "Get ingest URLs for a Streamplace station.",
      "parameters": {
        "type": "params",
        "properties": {}
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["ingests"],
          "properties": {
            "ingests": {
              "type": "array",
              "items": {
                "type": "union",
                "refs": ["place.stream.ingest.defs#ingest"]
              }
            }
          }
        }
      },
      "errors": []
    }
  }
}
```
