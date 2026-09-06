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
    setModerationPermissions: () => {},
    localLivestreamURI: null,
    setLocalLivestreamURI: () => {},
    ...overrides,
  } as LivestreamState;
}

const MODERATOR_DID = "did:plc:mod";
const OTHER_MODERATOR_DID = "did:plc:othermod";

function permissionRecord(overrides: Record<string, unknown> = {}) {
  return {
    $type: "place.stream.moderation.permission",
    moderator: MODERATOR_DID,
    permissions: ["ban", "hide"],
    createdAt: "2024-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function permissionView(record = permissionRecord()) {
  return {
    $type: "place.stream.moderation.defs#permissionView",
    uri: "at://did:plc:streamer/place.stream.moderation.permission/3kq",
    cid: "bafytest",
    author: { did: "did:plc:streamer", handle: "streamer.bsky.social" },
    record,
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
        permissionRecord({ moderator: OTHER_MODERATOR_DID }) as any,
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
        permissionRecord({ createdAt: "2023-06-01T00:00:00.000Z" }) as any,
      ],
    });
    const result = handleWebSocketMessages(state, [permissionView()]);
    expect(result.moderationPermissions).toHaveLength(2);
  });

  it("leaves moderationPermissions alone for unrelated messages", () => {
    const state = makeState();
    const result = handleWebSocketMessages(state, [
      { $type: "place.stream.livestream#viewerCount", count: 5 },
    ]);
    expect(result.moderationPermissions).toEqual([]);
  });
});
