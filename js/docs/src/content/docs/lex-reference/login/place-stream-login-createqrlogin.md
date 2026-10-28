---
title: place.stream.login.createQRLogin
description: Reference for the place.stream.login.createQRLogin lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `procedure`

Start a QR login: a browser that has made an OAuth pushed authorization request asks for a code that the Streamplace app, already signed in to this node, can scan and approve. The browser's user agent and IP address are recorded to show the approver.

**Parameters:** _(None defined)_

**Input:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name         | Type     | Req'd | Description                                                                                      | Constraints |
| ------------ | -------- | ----- | ------------------------------------------------------------------------------------------------ | ----------- |
| `requestUri` | `string` | ✅    | The request_uri the node's OAuth server returned for the browser's pushed authorization request. |             |

**Output:**

- **Encoding:** `application/json`
- **Schema:**

**Schema Type:** `object`

| Name        | Type     | Req'd | Description                                                                 | Constraints        |
| ----------- | -------- | ----- | --------------------------------------------------------------------------- | ------------------ |
| `id`        | `string` | ✅    | Identifies this login attempt to getQRLogin and completeQRLogin.            |                    |
| `url`       | `string` | ✅    | What the QR code encodes: the attempt's approval page, which the app opens. | Format: `uri`      |
| `expiresAt` | `string` | ✅    |                                                                             | Format: `datetime` |

**Possible Errors:**

- `QRLoginUnavailable`: This node has no Streamplace app configured to approve QR logins.

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.login.createQRLogin",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Start a QR login: a browser that has made an OAuth pushed authorization request asks for a code that the Streamplace app, already signed in to this node, can scan and approve. The browser's user agent and IP address are recorded to show the approver.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["requestUri"],
          "properties": {
            "requestUri": {
              "type": "string",
              "description": "The request_uri the node's OAuth server returned for the browser's pushed authorization request."
            }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["id", "url", "expiresAt"],
          "properties": {
            "id": {
              "type": "string",
              "description": "Identifies this login attempt to getQRLogin and completeQRLogin."
            },
            "url": {
              "type": "string",
              "format": "uri",
              "description": "What the QR code encodes: the attempt's approval page, which the app opens."
            },
            "expiresAt": {
              "type": "string",
              "format": "datetime"
            }
          }
        }
      },
      "errors": [
        {
          "name": "QRLoginUnavailable",
          "description": "This node has no Streamplace app configured to approve QR logins."
        }
      ]
    }
  }
}
```
