import { describe, expect, it, vi } from "vitest";
import {
  activeLiveCaptions,
  LIVE_CAPTION_LINGER_MS,
  presentedCaptionTime,
} from "../captions/live-cues";
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
    ingestProblems: [],
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
    captionTracks: [],
    liveCaptions: {},
    captionClock: null,
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

  it("applies unversioned revocations after a versioned grant", () => {
    const state = makeState({
      moderationPermissions: [permissionRecord() as any],
      moderationPermissionRevisions: { [PERMISSION_URI]: "3kqaaab" },
    });
    const result = handleWebSocketMessages(state, [deletedPermission()]);

    expect(result.moderationPermissions).toEqual([]);
    expect(result.deletedModerationPermissionURIs).toEqual([PERMISSION_URI]);
    expect(result.moderationPermissionRevisions).toEqual({
      [PERMISSION_URI]: "3kqaaab",
    });
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

  it("deduplicates live and initial-burst teleport arrivals", () => {
    const arrival = {
      $type: "place.stream.livestream#teleportArrival",
      teleportUri:
        "at://did:plc:source/place.stream.live.teleport/3lteleport0001",
      source: {
        did: "did:plc:source",
        handle: "source.example.com",
      },
      viewerCount: 15,
      startsAt: "2026-09-25T12:00:00.000Z",
    };

    const result = handleWebSocketMessages(makeState(), [
      arrival,
      {
        ...arrival,
        startsAt: "2026-09-25T12:00:00.123Z",
        viewerCount: 12,
      },
    ]);

    expect(result.chat).toHaveLength(1);
    expect(result.chat[0].record.text).toContain("15 viewers teleported");
  });
});

describe("handleWebSocketMessages: live captions", () => {
  const track = {
    id: "canonical-auto-en",
    language: "en",
    source: "auto",
    origin: "canonical",
  };
  const cue = (id: string, text: string, final: boolean) => ({
    $type: "place.stream.caption.defs#liveCue",
    id,
    streamer: "did:plc:streamer",
    track,
    startTime: "2026-09-25T12:00:00.000Z",
    endTime: "2026-09-25T12:00:01.000Z",
    text,
    final,
  });

  it("learns the track and keeps the latest revision of each cue", () => {
    const result = handleWebSocketMessages(makeState(), [
      cue("c1", "hello", false),
      cue("c1", "hello world", true),
      cue("c2", "next", false),
    ]);

    expect(result.captionTracks).toEqual([track]);
    const captions = Object.values(result.liveCaptions);
    expect(
      captions.find(
        (caption) => caption.id === "c1" && caption.trackId === track.id,
      ),
    ).toMatchObject({ text: "hello world", final: true });
    expect(
      captions.find(
        (caption) => caption.id === "c2" && caption.trackId === track.id,
      ),
    ).toBeDefined();
  });

  it("clears old tracks and cues when a livestream changes or ends", () => {
    const first = {
      $type: "place.stream.livestream#livestreamView",
      uri: "at://did:plc:streamer/place.stream.livestream/first",
      indexedAt: "2026-09-25T12:00:00.000Z",
      record: { title: "First", createdAt: "2026-09-25T12:00:00.000Z" },
    };
    let state = handleWebSocketMessages(makeState(), [
      first,
      cue("old", "Old speech", true),
    ]);
    state = handleWebSocketMessages(state, [
      { ...first, uri: first.uri.replace("first", "second") },
    ]);
    expect(state.captionTracks).toEqual([]);
    expect(state.liveCaptions).toEqual({});
    state = handleWebSocketMessages(state, [cue("new", "New speech", true)]);
    state = handleWebSocketMessages(state, [
      {
        ...first,
        uri: first.uri.replace("first", "second"),
        record: { ...first.record, endedAt: "2026-09-25T12:00:02.000Z" },
      },
    ]);
    expect(state.captionTracks).toEqual([]);
    expect(state.liveCaptions).toEqual({});
  });

  it("presents cues against segment time and ignores older replayed segments", () => {
    vi.useFakeTimers();
    try {
      vi.setSystemTime(new Date("2026-09-25T12:00:10.000Z"));
      const segment = {
        $type: "place.stream.segment",
        startTime: "2026-09-25T12:00:00.000Z",
      };
      let state = handleWebSocketMessages(makeState(), [
        segment,
        cue("speech", "Delayed speech", true),
      ]);
      const presented = presentedCaptionTime(state.captionClock, Date.now())!;
      expect(
        activeLiveCaptions(state.liveCaptions, track.id, presented).map(
          (c) => c.text,
        ),
      ).toEqual(["Delayed speech"]);
      vi.advanceTimersByTime(1000 + LIVE_CAPTION_LINGER_MS);
      expect(
        activeLiveCaptions(
          state.liveCaptions,
          track.id,
          presentedCaptionTime(state.captionClock, Date.now())!,
        ),
      ).toEqual([]);
      state = handleWebSocketMessages(state, [
        { ...segment, startTime: "2026-09-25T11:59:59.000Z" },
      ]);
      expect(presentedCaptionTime(state.captionClock, Date.now())).toBe(
        presented + 1000 + LIVE_CAPTION_LINGER_MS,
      );
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("handleWebSocketMessages ingest problems", () => {
  const deprecatedHost = {
    $type: "place.stream.ingest.defs#problem",
    code: "deprecated_ingest_host",
    message: "Your encoder is streaming to stream.place, an old address.",
    severity: "warning",
    link: "https://stream.place/docs/guides/start-streaming/obs/",
  };
  const problems = (list: unknown[]) => ({
    $type: "place.stream.ingest.defs#problems",
    problems: list,
  });
  const segment = (id: string) => ({
    $type: "place.stream.segment",
    id,
    signingKey: "did:key:zQ3sh",
    startTime: "2026-09-30T00:00:00.000Z",
    creator: "did:plc:streamer",
    duration: 1_000_000_000,
    video: [{ codec: "h264", width: 1280, height: 720, bframes: false }],
  });

  it("surfaces the node's problems", () => {
    const result = handleWebSocketMessages(makeState(), [
      problems([deprecatedHost]),
    ]);
    expect(result.problems).toEqual([
      {
        code: "deprecated_ingest_host",
        message: deprecatedHost.message,
        severity: "warning",
        link: deprecatedHost.link,
      },
    ]);
  });

  it("keeps them across segments, alongside segment problems", () => {
    const bframes = {
      ...segment("s1"),
      video: [{ codec: "h264", width: 1280, height: 720, bframes: true }],
    };
    const result = handleWebSocketMessages(makeState(), [
      problems([deprecatedHost]),
      bframes,
      segment("s2"),
    ]);
    expect(result.problems.map((p) => p.code)).toEqual([
      "bframes",
      "deprecated_ingest_host",
    ]);
  });

  it("clears them when the node sends an empty list", () => {
    const result = handleWebSocketMessages(makeState(), [
      problems([deprecatedHost]),
      segment("s1"),
      problems([]),
      segment("s2"),
    ]);
    expect(result.problems).toEqual([]);
    expect(result.ingestProblems).toEqual([]);
  });
});
