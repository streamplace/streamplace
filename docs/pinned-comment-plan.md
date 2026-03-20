# Pinned Comment Feature Plan

## Overview

Allow the streamer or a delegated moderator to pin an existing chat message. The pinned comment appears prominently in the chat UI and auto-expires when a TTL is reached or the stream ends.

## Design Decisions

- **Pinnable content**: Existing chat messages only (referenced by AT-URI)
- **Who can pin**: Streamer + mods with new `message.pin` permission
- **Expiration**: Optional TTL (`expiresAt` datetime) + auto-clear at stream end (client-side)
- **Storage pattern**: Record in streamer's AT Protocol repo, modeled after `place.stream.chat.gate`
- **Single active pin**: Only one pinned comment at a time. Creating a new pin replaces the previous one.
- **Viewer dismiss**: Any viewer can hide the pin from their own view (local state only). The pin remains for everyone else.

---

## 1. New Lexicon: `place.stream.chat.pinnedRecord`

**File**: `lexicons/place/stream/chat/pinnedRecord.json`

```json
{
  "lexicon": 1,
  "id": "place.stream.chat.pinnedRecord",
  "defs": {
    "main": {
      "type": "record",
      "key": "tid",
      "description": "Record pinning a chat message for prominent display.",
      "record": {
        "type": "object",
        "required": ["pinnedMessage", "createdAt"],
        "properties": {
          "pinnedMessage": {
            "type": "string",
            "format": "at-uri",
            "description": "AT-URI of the pinned chat message."
          },
          "createdAt": {
            "type": "string",
            "format": "datetime",
            "description": "When this pin was created."
          },
          "expiresAt": {
            "type": "string",
            "format": "datetime",
            "description": "Optional expiration time. If set, the pin is considered inactive after this time."
          }
        }
      }
    }
  }
}
```

---

## 2. Pinned Record View + Defs

### 2a. Add `pinnedRecordView` to chat defs

**File**: `lexicons/place/stream/chat/defs.json` (add new definition)

```json
"pinnedRecordView": {
  "type": "object",
  "description": "View of a pinned chat record with hydrated message data.",
  "required": ["uri", "cid", "record", "indexedAt"],
  "properties": {
    "uri": { "type": "string", "format": "at-uri" },
    "cid": { "type": "string", "format": "cid" },
    "record": { "type": "ref", "ref": "place.stream.chat.pinnedRecord" },
    "indexedAt": { "type": "string", "format": "datetime" },
    "pinnedBy": { "type": "ref", "ref": "app.bsky.actor.defs#profileViewBasic" },
    "message": { "type": "ref", "ref": "place.stream.chat.defs#messageView" }
  }
}
```

This gives us a hydrated view that includes the full message data, so the frontend doesn't need to look it up from the chat index.

### 2b. Go struct for bus delivery

The bus will deliver a `PinnedRecordView` (not just the raw record) so the frontend can render the message text directly. Similar to how `ChatGate` records are published raw but the frontend processes them.

---

## 3. New Permission: `message.pin`

### 3a. Add to lexicon enum

**File**: `lexicons/place/stream/moderation/permission.json`

Add `"message.pin"` to the permissions enum:

```json
"enum": ["ban", "hide", "livestream.manage", "message.pin"]
```

### 3b. Register in Go permission system

**File**: `pkg/moderation/permissions.go`

```go
const PermissionMessagePin = "message.pin"

var ActionPermissions = map[string]string{
    // ... existing entries ...
    "createPin": PermissionMessagePin,
    "deletePin": PermissionMessagePin,
}
```

### 3c. Update frontend permissions

**File**: `js/components/src/streamplace-store/moderation.tsx`

Add `canPin: boolean` to `ModerationPermissions` interface. Derive from permissions array:

```ts
canPin: isOwner || permissions.includes("message.pin"),
```

---

## 4. New RPC Procedures

### 4a. Lexicon: `place.stream.moderation.createPin`

**File**: `lexicons/place/stream/moderation/createPin.json`

```json
{
  "lexicon": 1,
  "id": "place.stream.moderation.createPin",
  "defs": {
    "main": {
      "type": "procedure",
      "description": "Pin a chat message on behalf of a streamer. Requires 'message.pin' permission. Creates a place.stream.chat.pinnedRecord in the streamer's repo, replacing any existing pin.",
      "input": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["streamer", "messageUri"],
          "properties": {
            "streamer": { "type": "string", "format": "did" },
            "messageUri": { "type": "string", "format": "at-uri" },
            "expiresAt": { "type": "string", "format": "datetime" }
          }
        }
      },
      "output": {
        "encoding": "application/json",
        "schema": {
          "type": "object",
          "required": ["uri", "cid"],
          "properties": {
            "uri": { "type": "string", "format": "at-uri" },
            "cid": { "type": "string", "format": "cid" }
          }
        }
      },
      "errors": [
        { "name": "Unauthorized" },
        { "name": "Forbidden" },
        { "name": "SessionNotFound" }
      ]
    }
  }
}
```

### 4b. Lexicon: `place.stream.moderation.deletePin`

**File**: `lexicons/place/stream/moderation/deletePin.json`

Same pattern as `deleteGate`:

- Input: `streamer` (DID), `pinUri` (at-uri)
- Output: empty
- Errors: Unauthorized, Forbidden, SessionNotFound

### 4c. Go handlers

**File**: `pkg/spxrpc/place_stream_moderation.go`

Add `handlePlaceStreamModerationCreatePin`:

1. Validate input (DID, AT-URI)
2. `GetDelegatedModerationContext(ctx, input.Streamer, "createPin")`
3. Before creating: list existing `place.stream.chat.pinnedRecord` records in streamer's repo, delete any existing ones (single-pin semantics)
4. Build `streamplace.ChatPinnedRecord` struct
5. Create via `com.atproto.repo.createRecord` on streamer's repo
6. Audit log
7. Return URI + CID

Add `handlePlaceStreamModerationDeletePin`:

1. Validate input
2. `GetDelegatedModerationContext(ctx, input.Streamer, "deletePin")`
3. Extract rkey, delete record via `com.atproto.repo.deleteRecord`
4. Audit log

### 4d. Register handlers

In the XRPC server setup where `createGate`/`deleteGate` are registered, add `createPin` and `deletePin`.

---

## 5. Backend: Model + DB

**File**: `pkg/model/pinned_record.go`

```go
type PinnedRecord struct {
    RKey          string     `gorm:"primaryKey;column:rkey"`
    CID           string     `gorm:"column:cid"`
    RepoDID       string     `gorm:"column:repo_did"`
    Repo          *Repo      `gorm:"foreignKey:DID;references:RepoDID"`
    PinnedMessage string     `gorm:"column:pinned_message"`
    ExpiresAt     *time.Time `gorm:"column:expires_at"`
    CreatedAt     time.Time  `gorm:"column:created_at"`
}
```

Methods:

- `CreatePinnedRecord(ctx, pin)` - insert
- `GetPinnedRecord(ctx, rkey)` - single lookup
- `DeletePinnedRecord(ctx, rkey)` - delete by rkey
- `GetActivePinnedRecord(ctx, streamerDID)` - returns the most recent non-expired pin for a streamer
- `DeleteAllPinnedRecords(ctx, streamerDID)` - bulk delete (called before creating new pin)

Add `PinnedRecord{}` to the AutoMigrate list in `pkg/model/model.go`.

---

## 6. Constants + Firehose

### 6a. Constants

**File**: `pkg/constants/constants.go`

Add: `var PLACE_STREAM_CHAT_PINNED_RECORD = "place.stream.chat.pinnedRecord"`

### 6b. Sync (create/update)

**File**: `pkg/atproto/sync.go` - `handleCreateUpdate`

Add case `*streamplace.ChatPinnedRecord`:

- Sync bluesky repo
- Delete existing pinned records for this streamer (single-pin enforcement at DB level)
- Create new `PinnedRecord` model entry
- Build a hydrated view (resolve the pinned message, include author info) and publish to bus on streamer's channel

### 6c. Firehose (delete)

**File**: `pkg/atproto/firehose.go` - `EvtKindDeleteRecord`

Add handling for `constants.PLACE_STREAM_CHAT_PINNED_RECORD`:

- `DeletePinnedRecord(ctx, rkey)`
- Publish deletion marker to bus

---

## 7. Bus / WebSocket Delivery

### Create event

Publish a `PinnedRecordView`-like object to the streamer's bus channel:

```json
{
  "$type": "place.stream.chat.defs#pinnedRecordView",
  "uri": "at://...",
  "cid": "...",
  "record": {
    "pinnedMessage": "at://...",
    "createdAt": "...",
    "expiresAt": "..."
  },
  "indexedAt": "...",
  "message": {
    /* hydrated messageView with author, text, facets, etc */
  }
}
```

### Delete event

Publish a deletion marker:

```json
{
  "$type": "place.stream.chat.pinnedRecord",
  "deleted": true,
  "rkey": "..."
}
```

---

## 8. Frontend State

### LivestreamState additions

**File**: `js/components/src/livestream-store/livestream-state.tsx`

```ts
pinnedComment: PinnedRecordView | null;
```

Where `PinnedRecordView` is a type with the hydrated message + pin metadata.

### Store init

**File**: `js/components/src/livestream-store/livestream-store.tsx`

Add `pinnedComment: null` to initial state.

### Chat hooks

**File**: `js/components/src/livestream-store/chat.tsx`

Add:

- `usePinnedComment()` - selector: `state.pinnedComment`
- `usePinChatMessage()` - hook to pin a message:
  - If streamer: direct `com.atproto.repo.createRecord` for `place.stream.chat.pinnedRecord`
  - If mod: call `place.stream.moderation.createPin` XRPC
  - On success, the bus event will update state automatically
- `useUnpinChatMessage()` - hook to unpin:
  - If streamer: direct `com.atproto.repo.deleteRecord`
  - If mod: call `place.stream.moderation.deletePin` XRPC

### WebSocket consumer

**File**: `js/components/src/livestream-store/websocket-consumer.tsx`

Add handler for `place.stream.chat.defs#pinnedRecordView`:

- Set `state.pinnedComment` to the hydrated view

Add handler for `place.stream.chat.pinnedRecord` with `deleted: true`:

- Set `state.pinnedComment` to `null`

### Expiration

In the Chat component, `useEffect` checks `pinnedComment.record.expiresAt`. If current time passes it, clear `pinnedComment` locally. Also clear when `livestream.record.endedAt` is set (stream ended).

---

## 9. Frontend UI

### Pinned comment as persistent stream notification

The pinned comment uses the existing `StreamNotificationProvider` + `streamNotificationManager` system.

**File**: `js/components/src/components/stream-notification/pinned-comment-notification.tsx` (new)

A custom render function passed to `streamNotificationManager.show()` with `duration: 0` (manual dismiss only), making it a persistent notification that sits at the top of the notification stack. Other temporary notifications (teleport, etc.) will appear above/below it and dismiss independently.

The render function receives `(isExiting, onDismiss, startTime)` and renders:

- Pin icon + author name (with chatProfile color) + message text (with facets rendered)
- Two dismiss actions:
  - **Unpin** (X icon, only visible if `canPin`): calls `useUnpinChatMessage()`, removes the pin globally, then calls `onDismiss("user")` to dismiss the notification
  - **Hide** (eye-off icon, visible to all viewers): calls `onDismiss("user")` to dismiss the notification locally only. Does NOT call the server - the pin remains for other viewers
- Does NOT link to or scroll to the original message (chat history is limited)
- Auto-dismisses when `expiresAt` is reached (a `useEffect` watching the pinned comment state calls `streamNotificationManager.requestDismiss("pinned-comment", "auto")`)
- Auto-dismisses when stream ends

**Integration point**: The `TeleportWatcher` / livestream provider already manages notification lifecycle. The pinned comment notification is managed similarly - when `state.pinnedComment` changes in the Zustand store, a hook or effect calls `streamNotificationManager.show()` to create/update the notification, or `streamNotificationManager.hide()` to remove it.

The notification ID should be fixed (e.g., `"pinned-comment"`) so that replacing a pin updates the same notification rather than creating a new one.

### Pin action in mod menu

**File**: `js/components/src/components/chat/mod-view.tsx`

Add "Pin this message" in the moderation actions group (visible when `canPin` is true):

```tsx
{
  modPermissions.canPin && (
    <DropdownMenuItem
      onPress={() => {
        /* pin message */
      }}
    >
      <Text>Pin this message</Text>
    </DropdownMenuItem>
  );
}
```

---

## 10. Moderator Panel

**File**: `js/components/src/components/dashboard/moderator-panel.tsx`

Add `"message.pin"` to the list of assignable permissions when adding/editing moderators.

---

## 11. Code Generation

After creating lexicon JSON files, run `lexgen` to generate Go types:

- `pkg/streamplace/chatpinnedrecord.go` (auto-generated)
- Update generated TypeScript types in the `streamplace` package

---

## File Change Summary

| File                                                                               | Action                                                  |
| ---------------------------------------------------------------------------------- | ------------------------------------------------------- |
| `lexicons/place/stream/chat/pinnedRecord.json`                                     | **New**                                                 |
| `lexicons/place/stream/chat/defs.json`                                             | **Edit** - add `pinnedRecordView`                       |
| `lexicons/place/stream/moderation/createPin.json`                                  | **New**                                                 |
| `lexicons/place/stream/moderation/deletePin.json`                                  | **New**                                                 |
| `lexicons/place/stream/moderation/permission.json`                                 | **Edit** - add `message.pin`                            |
| `pkg/constants/constants.go`                                                       | **Edit**                                                |
| `pkg/moderation/permissions.go`                                                    | **Edit**                                                |
| `pkg/model/pinned_record.go`                                                       | **New**                                                 |
| `pkg/model/model.go`                                                               | **Edit**                                                |
| `pkg/atproto/sync.go`                                                              | **Edit**                                                |
| `pkg/atproto/firehose.go`                                                          | **Edit**                                                |
| `pkg/spxrpc/place_stream_moderation.go`                                            | **Edit**                                                |
| `js/components/src/streamplace-store/moderation.tsx`                               | **Edit**                                                |
| `js/components/src/streamplace-store/block.tsx`                                    | **Edit** - add pin/unpin hooks                          |
| `js/components/src/livestream-store/livestream-state.tsx`                          | **Edit**                                                |
| `js/components/src/livestream-store/livestream-store.tsx`                          | **Edit**                                                |
| `js/components/src/livestream-store/chat.tsx`                                      | **Edit**                                                |
| `js/components/src/livestream-store/websocket-consumer.tsx`                        | **Edit**                                                |
| `js/components/src/components/stream-notification/pinned-comment-notification.tsx` | **New** - notification render function                  |
| `js/components/src/livestream-provider/index.tsx`                                  | **Edit** - manage pinned comment notification lifecycle |
| `js/components/src/components/chat/mod-view.tsx`                                   | **Edit** - add pin action                               |
| `js/components/src/components/dashboard/moderator-panel.tsx`                       | **Edit**                                                |

---

## Implementation Order

1. Lexicons (pinnedRecord, createPin, deletePin, permission, defs)
2. Code generation (lexgen)
3. Backend: constants, permissions, model, DB migration
4. Backend: sync/firehose integration
5. Backend: XRPC handlers + registration
6. Frontend: state types, store init, hooks
7. Frontend: WebSocket consumer handlers
8. Frontend: pinned-comment notification render function + lifecycle management
9. Frontend: mod-view pin action
10. Frontend: moderator panel permission
