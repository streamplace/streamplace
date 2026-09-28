import { describe, expect, it } from "vitest";
import type { LivestreamState } from "./state";
import { handleWebSocketMessages } from "./websocket-consumer";

function makeState(overrides: Partial<LivestreamState> = {}): LivestreamState {
  return {
    profile: null,
    chatIndex: {},
    chat: [],
    authors: {},
    livestream: null,
    viewers: null,
    pendingHides: [],
    segment: null,
    recentSegments: [],
    problems: [],
    renditions: [],
    replyToMessage: null,
    chatDraft: "",
    badgeSlots: null,
    streamKey: null,
    setStreamKey: () => {},
    activeTeleport: null,
    activeTeleportUri: null,
    setActiveTeleportUri: () => {},
    websocketConnected: false,
    hasReceivedSegment: false,
    pinnedComment: null,
    moderationPermissions: [],
    deletedModerationPermissionURIs: [],
    moderationPermissionRevisions: {},
    setModerationPermissions: () => {},
    localLivestreamURI: null,
    setLocalLivestreamURI: () => {},
    ...overrides,
  } as LivestreamState;
}

const MODERATOR_DID = "did:plc:mod";
const OTHER_MODERATOR_DID = "did:plc:othermod";
const PERMISSION_URI =
  "at://did:plc:streamer/place.stream.moderation.permission/3kq";
const OTHER_PERMISSION_URI =
  "at://did:plc:streamer/place.stream.moderation.permission/3kz";

function permissionRecord(overrides: Record<string, unknown> = {}) {
  return {
    $type: "place.stream.moderation.permission",
    moderator: MODERATOR_DID,
    permissions: ["ban", "hide"],
    createdAt: "2024-01-01T00:00:00.000Z",
    uri: PERMISSION_URI,
    ...overrides,
  };
}

function permissionView(record = permissionRecord(), repoRev?: string) {
  return {
    $type: "place.stream.moderation.defs#permissionView",
    uri: PERMISSION_URI,
    cid: "bafytest",
    repoRev,
    author: { did: "did:plc:streamer", handle: "streamer.bsky.social" },
    record,
  };
}

function deletedPermission(
  overrides: Record<string, unknown> = {},
  repoRev?: string,
) {
  return {
    $type: "place.stream.moderation.permission",
    deleted: true,
    uri: PERMISSION_URI,
    repoRev,
    ...overrides,
  };
}

describe("handleWebSocketMessages: moderation permission views", () => {
  it("merges a permission view into an empty store", () => {
    const state = makeState();
    const result = handleWebSocketMessages(state, [permissionView()]);
    expect(result.moderationPermissions).toHaveLength(1);
    expect(result.moderationPermissions[0].moderator).toBe(MODERATOR_DID);
    expect(result.moderationPermissions[0].permissions).toEqual([
      "ban",
      "hide",
    ]);
    expect(result.moderationPermissions[0].uri).toBe(PERMISSION_URI);
  });

  it("replaces the record fetched earlier for the same moderator and createdAt", () => {
    const state = makeState({
      moderationPermissions: [permissionRecord() as any],
    });
    const result = handleWebSocketMessages(state, [
      permissionView(permissionRecord({ permissions: ["ban"] })),
    ]);
    expect(result.moderationPermissions).toHaveLength(1);
    expect(result.moderationPermissions[0].permissions).toEqual(["ban"]);
  });

  it("keeps records belonging to other moderators", () => {
    const state = makeState({
      moderationPermissions: [
        permissionRecord({
          moderator: OTHER_MODERATOR_DID,
          uri: OTHER_PERMISSION_URI,
        }) as any,
      ],
    });
    const result = handleWebSocketMessages(state, [permissionView()]);
    expect(result.moderationPermissions).toHaveLength(2);
    expect(result.moderationPermissions.map((r) => r.moderator)).toEqual([
      OTHER_MODERATOR_DID,
      MODERATOR_DID,
    ]);
  });

  it("treats records with a different createdAt as separate delegations", () => {
    const state = makeState({
      moderationPermissions: [
        permissionRecord({
          createdAt: "2023-06-01T00:00:00.000Z",
          uri: OTHER_PERMISSION_URI,
        }) as any,
      ],
    });
    const result = handleWebSocketMessages(state, [permissionView()]);
    expect(result.moderationPermissions).toHaveLength(2);
  });

  it("replaces an updated permission view for the same URI", () => {
    const state = makeState({
      moderationPermissions: [
        permissionRecord({ permissions: ["ban"] }) as any,
      ],
    });
    const result = handleWebSocketMessages(state, [
      permissionView(
        permissionRecord({
          permissions: ["hide"],
          createdAt: "2024-01-02T00:00:00.000Z",
        }),
      ),
    ]);

    expect(result.moderationPermissions).toHaveLength(1);
    expect(result.moderationPermissions[0].permissions).toEqual(["hide"]);
  });

  it("removes only the permission named by a deletion event", () => {
    const state = makeState({
      moderationPermissions: [
        permissionRecord() as any,
        permissionRecord({
          moderator: OTHER_MODERATOR_DID,
          uri: OTHER_PERMISSION_URI,
        }) as any,
      ],
    });
    const result = handleWebSocketMessages(state, [deletedPermission()]);

    expect(result.moderationPermissions).toHaveLength(1);
    expect(result.moderationPermissions[0].moderator).toBe(OTHER_MODERATOR_DID);
    expect(result.deletedModerationPermissionURIs).toContain(PERMISSION_URI);
  });

  it("remembers deletions that arrive before the permission is loaded", () => {
    const result = handleWebSocketMessages(makeState(), [deletedPermission()]);

    expect(result.moderationPermissions).toEqual([]);
    expect(result.deletedModerationPermissionURIs).toEqual([PERMISSION_URI]);
  });

  it("clears a deletion marker when the same record is recreated", () => {
    const state = makeState({
      deletedModerationPermissionURIs: [PERMISSION_URI],
      moderationPermissionRevisions: { [PERMISSION_URI]: "3kqaaaa" },
    });
    const result = handleWebSocketMessages(state, [
      permissionView(permissionRecord(), "3kqaaab"),
    ]);

    expect(result.deletedModerationPermissionURIs).toEqual([]);
    expect(result.moderationPermissions).toHaveLength(1);
  });

  it("ignores an older permission event delivered after its deletion", () => {
    const deleted = handleWebSocketMessages(makeState(), [
      deletedPermission({}, "3kqaaab"),
    ]);
    const result = handleWebSocketMessages(deleted, [
      permissionView(permissionRecord(), "3kqaaaa"),
    ]);

    expect(result.moderationPermissions).toEqual([]);
    expect(result.deletedModerationPermissionURIs).toEqual([PERMISSION_URI]);
    expect(result.moderationPermissionRevisions).toEqual({
      [PERMISSION_URI]: "3kqaaab",
    });
  });

  it("accepts a newer recreation and ignores a delayed older deletion", () => {
    const deleted = handleWebSocketMessages(makeState(), [
      deletedPermission({}, "3kqaaaa"),
    ]);
    const recreated = handleWebSocketMessages(deleted, [
      permissionView(permissionRecord(), "3kqaaab"),
    ]);
    const result = handleWebSocketMessages(recreated, [
      deletedPermission({}, "3kqaaaa"),
    ]);

    expect(result.moderationPermissions).toHaveLength(1);
    expect(result.deletedModerationPermissionURIs).toEqual([]);
    expect(result.moderationPermissionRevisions).toEqual({
      [PERMISSION_URI]: "3kqaaab",
    });
  });

  it("keeps an unversioned deletion fail-closed", () => {
    const deleted = handleWebSocketMessages(makeState(), [deletedPermission()]);
    const result = handleWebSocketMessages(deleted, [
      permissionView(permissionRecord(), "3kqaaab"),
    ]);

    expect(result.moderationPermissions).toEqual([]);
    expect(result.deletedModerationPermissionURIs).toEqual([PERMISSION_URI]);
  });

  it("supports the server deletion marker identified by streamer and rkey", () => {
    const state = makeState({
      moderationPermissions: [permissionRecord() as any],
    });
    const result = handleWebSocketMessages(state, [
      deletedPermission({
        uri: undefined,
        streamer: "did:plc:streamer",
        rkey: "3kq",
      }),
    ]);

    expect(result.moderationPermissions).toEqual([]);
  });

  it("leaves moderationPermissions alone for unrelated messages", () => {
    const state = makeState();
    const result = handleWebSocketMessages(state, [
      { $type: "place.stream.livestream#viewerCount", count: 5 },
    ]);
    expect(result.moderationPermissions).toEqual([]);
  });
});

describe("handleWebSocketMessages: teleport records", () => {
  it("stores incoming teleport records as the active teleport", () => {
    const teleport = {
      $type: "place.stream.live.teleport",
      destination: "did:plc:destination",
      createdAt: "2024-01-01T00:00:00.000Z",
    };

    const result = handleWebSocketMessages(makeState(), [teleport]);

    expect(result.activeTeleport).toEqual(teleport);
  });
});
