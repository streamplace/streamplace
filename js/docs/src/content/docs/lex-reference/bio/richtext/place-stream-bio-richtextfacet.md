---
title: place.stream.bio.richtextFacet
description: Reference for the place.stream.bio.richtextFacet lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `object`

Annotation of a sub-string within rich text.

**Properties:**

| Name       | Type                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Req'd | Description | Constraints |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----------- | ----------- |
| `index`    | [`#byteSlice`](#byteslice)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | ✅    |             |             |
| `features` | Array of Union of:<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#mention`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#mention)<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#link`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#link)<br/>&nbsp;&nbsp;[`app.bsky.richtext.facet#tag`](https://github.com/bluesky-social/atproto/tree/main/lexicons/app/bsky/richtext/facet.json#tag)<br/>&nbsp;&nbsp;[`#id`](#id)<br/>&nbsp;&nbsp;[`#bold`](#bold)<br/>&nbsp;&nbsp;[`#italic`](#italic)<br/>&nbsp;&nbsp;[`#code`](#code)<br/>&nbsp;&nbsp;[`#highlight`](#highlight)<br/>&nbsp;&nbsp;[`#underline`](#underline)<br/>&nbsp;&nbsp;[`#strikethrough`](#strikethrough)<br/>&nbsp;&nbsp;[`#link`](#link)<br/>&nbsp;&nbsp;[`#atMention`](#atmention)<br/>&nbsp;&nbsp;[`#didMention`](#didmention) | ✅    |             |             |

---

<a name="id"></a>

### `id`

**Type:** `object`

Facet feature for an identifier. Used for linking to a segment

**Properties:**

| Name | Type     | Req'd | Description | Constraints |
| ---- | -------- | ----- | ----------- | ----------- |
| `id` | `string` | ❌    |             |             |

---

<a name="bold"></a>

### `bold`

**Type:** `object`

Facet feature for bold text

**Properties:**

_(No properties defined)_

---

<a name="italic"></a>

### `italic`

**Type:** `object`

Facet feature for italic text

**Properties:**

_(No properties defined)_

---

<a name="code"></a>

### `code`

**Type:** `object`

Facet feature for inline code.

**Properties:**

_(No properties defined)_

---

<a name="highlight"></a>

### `highlight`

**Type:** `object`

Facet feature for highlighted text.

**Properties:**

_(No properties defined)_

---

<a name="underline"></a>

### `underline`

**Type:** `object`

Facet feature for underline markup

**Properties:**

_(No properties defined)_

---

<a name="strikethrough"></a>

### `strikethrough`

**Type:** `object`

Facet feature for strikethrough markup

**Properties:**

_(No properties defined)_

---

<a name="link"></a>

### `link`

**Type:** `object`

Facet feature for a URL. The text URL may have been simplified or truncated, but the facet reference should be a complete URL.

**Properties:**

| Name  | Type     | Req'd | Description | Constraints |
| ----- | -------- | ----- | ----------- | ----------- |
| `uri` | `string` | ✅    |             |             |

---

<a name="atmention"></a>

### `atMention`

**Type:** `object`

Facet feature for mentioning an AT URI.

**Properties:**

| Name    | Type     | Req'd | Description | Constraints   |
| ------- | -------- | ----- | ----------- | ------------- |
| `atURI` | `string` | ✅    |             | Format: `uri` |
| `href`  | `string` | ❌    |             | Format: `uri` |

---

<a name="didmention"></a>

### `didMention`

**Type:** `object`

Facet feature for mentioning a did.

**Properties:**

| Name  | Type     | Req'd | Description | Constraints   |
| ----- | -------- | ----- | ----------- | ------------- |
| `did` | `string` | ✅    |             | Format: `did` |

---

<a name="byteslice"></a>

### `byteSlice`

**Type:** `object`

Specifies the sub-string range a facet feature applies to.

**Properties:**

| Name        | Type      | Req'd | Description | Constraints |
| ----------- | --------- | ----- | ----------- | ----------- |
| `byteStart` | `integer` | ✅    |             | Min: 0      |
| `byteEnd`   | `integer` | ✅    |             | Min: 0      |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.bio.richtextFacet",
  "defs": {
    "main": {
      "type": "object",
      "description": "Annotation of a sub-string within rich text.",
      "required": ["index", "features"],
      "properties": {
        "index": {
          "type": "ref",
          "ref": "#byteSlice"
        },
        "features": {
          "type": "array",
          "items": {
            "type": "union",
            "refs": [
              "app.bsky.richtext.facet#mention",
              "app.bsky.richtext.facet#link",
              "app.bsky.richtext.facet#tag",
              "#id",
              "#bold",
              "#italic",
              "#code",
              "#highlight",
              "#underline",
              "#strikethrough",
              "#link",
              "#atMention",
              "#didMention"
            ]
          }
        }
      }
    },
    "id": {
      "type": "object",
      "description": "Facet feature for an identifier. Used for linking to a segment",
      "required": [],
      "properties": {
        "id": {
          "type": "string"
        }
      }
    },
    "bold": {
      "type": "object",
      "description": "Facet feature for bold text",
      "required": [],
      "properties": {}
    },
    "italic": {
      "type": "object",
      "description": "Facet feature for italic text",
      "required": [],
      "properties": {}
    },
    "code": {
      "type": "object",
      "description": "Facet feature for inline code.",
      "required": [],
      "properties": {}
    },
    "highlight": {
      "type": "object",
      "description": "Facet feature for highlighted text.",
      "required": [],
      "properties": {}
    },
    "underline": {
      "type": "object",
      "description": "Facet feature for underline markup",
      "required": [],
      "properties": {}
    },
    "strikethrough": {
      "type": "object",
      "description": "Facet feature for strikethrough markup",
      "required": [],
      "properties": {}
    },
    "link": {
      "type": "object",
      "description": "Facet feature for a URL. The text URL may have been simplified or truncated, but the facet reference should be a complete URL.",
      "required": ["uri"],
      "properties": {
        "uri": {
          "type": "string"
        }
      }
    },
    "atMention": {
      "type": "object",
      "description": "Facet feature for mentioning an AT URI.",
      "required": ["atURI"],
      "properties": {
        "atURI": {
          "type": "string",
          "format": "uri"
        },
        "href": {
          "type": "string",
          "format": "uri"
        }
      }
    },
    "didMention": {
      "type": "object",
      "description": "Facet feature for mentioning a did.",
      "required": ["did"],
      "properties": {
        "did": {
          "type": "string",
          "format": "did"
        }
      }
    },
    "byteSlice": {
      "type": "object",
      "description": "Specifies the sub-string range a facet feature applies to.",
      "required": ["byteStart", "byteEnd"],
      "properties": {
        "byteStart": {
          "type": "integer",
          "minimum": 0
        },
        "byteEnd": {
          "type": "integer",
          "minimum": 0
        }
      }
    }
  }
}
```
