---
title: Chat Popout
description: Open chat in a separate browser window with customizable options
---

# Chat Popout

The chat popout feature lets you open chat in a separate browser window, useful for monitoring chat while doing other activities on your computer.

## Basic Usage

To open chat in a popout, navigate to:

```
https://streamplace-node.url/chat-popout/{user-did}
```

For example:

```
https://stream.place/chat-popout/did:plc:abcdef123456...
```

## Query Parameters

The chat popout supports several URL parameters to customize its behavior, primarily to customise as an OBS browser source:

### `reverse`

Reverses the chat order, showing newest messages at the bottom (standard chat order) instead of at the top.

- Type: `boolean` (values: `"true"`, `"false"`)
- Default: `"false"`

Example:

```
https://stream.place/chat-popout/did:plc:abcdef123456...?reverse=true
```

### `hideAfter`

Automatically hides the chat after a specified number of seconds. This is useful for temporary chat displays or overlays.

- Type: `number` (in seconds)
- Default: not set

Example:

```
https://stream.place/chat-popout/did:plc:abcdef123456...?hideAfter=30
```

### `hideChatBox`

Hides the chat input box, making the popout read-only. Useful when you want to display chat on stream without allowing interaction.

- Type: `boolean` (values: `"true"`, `"false"`)
- Default: `"false"`

Example:

```
https://stream.place/chat-popout/did:plc:abcdef123456...?hideChatBox=true
```

### `hidePinnedComments`

Hides pinned comment notifications in the popout.

- Type: `boolean` (values: `"true"`, `"false"`)
- Default: `"false"`

Example:

```
https://stream.place/chat-popout/did:plc:abcdef123456...?hidePinnedComments=true
```
