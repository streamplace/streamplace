import type { ChatMessageViewHydrated } from "streamplace";
import { describe, expect, it } from "vitest";
import { reduceChat } from "./chat-reducer";
import type { LivestreamState } from "./state";

function makeMsg(
  overrides: Partial<ChatMessageViewHydrated> & {
    uri?: string;
    did?: string;
    text?: string;
    createdAt?: string;
  },
): ChatMessageViewHydrated {
  const uri =
    overrides.uri ??
    `at://did:plc:test/app.bsky.feed.post/${Math.random().toString(36).slice(2)}`;
  const did = overrides.did ?? "did:plc:user1";
  const text = overrides.text ?? "hello";
  const createdAt = overrides.createdAt ?? "2024-01-01T00:00:00.000Z";
  return {
    uri,
    cid: "cid-" + uri,
    author: { did, handle: did + ".bsky.social" },
    record: {
      $type: "place.stream.chat.message",
      text,
      createdAt,
      streamer: "did:plc:streamer",
    },
    indexedAt: createdAt,
    chatProfile: { color: { red: 255, green: 255, blue: 255 } },
    ...overrides,
    author: overrides.author ?? { did, handle: did + ".bsky.social" },
    record: overrides.record ?? {
      $type: "place.stream.chat.message",
      text,
      createdAt,
      streamer: "did:plc:streamer",
    },
  } as ChatMessageViewHydrated;
}

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

describe("reduceChat", () => {
  it("returns state unchanged for empty delta", () => {
    const state = makeState();
    const result = reduceChat(state, [], [], []);
    expect(result).toBe(state);
  });

  it("adds a new message to the chat list", () => {
    const state = makeState();
    const msg = makeMsg({ text: "hello world" });
    const result = reduceChat(state, [msg], [], []);
    expect(result.chat).toHaveLength(1);
    expect(result.chat[0].record.text).toBe("hello world");
    const key = Object.keys(result.chatIndex)[0];
    expect(key).toContain("1704067200000");
  });

  it("sorts messages by timestamp", () => {
    const state = makeState();
    const early = makeMsg({
      text: "first",
      createdAt: "2024-01-01T00:00:01.000Z",
      uri: "at://a/1",
    });
    const late = makeMsg({
      text: "second",
      createdAt: "2024-01-01T00:00:02.000Z",
      uri: "at://a/2",
    });
    const result = reduceChat(state, [late, early], [], []);
    expect(result.chat[0].record.text).toBe("first");
    expect(result.chat[1].record.text).toBe("second");
  });

  it("skips messages already in the index", () => {
    const msg = makeMsg({ text: "dupe" });
    const key = "1704067200000-" + msg.uri;
    const state = makeState({ chatIndex: { [key]: msg }, chat: [msg] });
    const result = reduceChat(state, [msg], [], []);
    expect(result.chat).toHaveLength(1);
  });

  it("removes messages from blocked users", () => {
    const blockedDid = "did:plc:blocked";
    const msg = makeMsg({ did: blockedDid, text: "I am blocked" });
    const blockedKey = "1704067200000-" + msg.uri;
    const state = makeState({
      chatIndex: { [blockedKey]: msg },
      chat: [msg],
    });
    const result = reduceChat(
      state,
      [],
      [
        {
          record: {
            subject: blockedDid,
            createdAt: "2024-01-01",
            $type: "app.bsky.graph.block",
          },
        } as any,
      ],
      [],
    );
    expect(result.chat).toHaveLength(0);
    expect(Object.keys(result.chatIndex)).toHaveLength(0);
  });

  it("hides messages by URI", () => {
    const msg = makeMsg({ text: "hide me", uri: "at://hide/1" });
    const state = makeState({
      chatIndex: { "1704067200000-at://hide/1": msg },
      chat: [msg],
    });
    const result = reduceChat(state, [], [], ["at://hide/1"]);
    expect(result.chat).toHaveLength(0);
    expect(Object.keys(result.chatIndex)).toHaveLength(0);
  });

  it("replaces local messages with real ones on same text+author within 10s", () => {
    const localMsg = makeMsg({
      uri: "local-1704067200000",
      text: "hello",
      createdAt: "2024-01-01T00:00:00.000Z",
    });
    const realMsg = makeMsg({
      uri: "at://real/1",
      text: "hello",
      createdAt: "2024-01-01T00:00:05.000Z",
    });
    const state = makeState({
      chatIndex: { "1704067200000-local-1704067200000": localMsg },
      chat: [localMsg],
    });
    const result = reduceChat(state, [realMsg], [], []);
    expect(result.chat).toHaveLength(1);
    expect(result.chat[0].uri).toBe("at://real/1");
  });

  it("does not replace local messages when text differs", () => {
    const localMsg = makeMsg({
      uri: "local-1",
      text: "hello",
      createdAt: "2024-01-01T00:00:00.000Z",
    });
    const realMsg = makeMsg({
      uri: "at://real/1",
      text: "different text",
      createdAt: "2024-01-01T00:00:05.000Z",
    });
    const state = makeState({
      chatIndex: { "1704067200000-local-1": localMsg },
      chat: [localMsg],
    });
    const result = reduceChat(state, [realMsg], [], []);
    expect(result.chat).toHaveLength(2);
  });

  it("removes deleted messages", () => {
    const msg = makeMsg({ text: "delete me", uri: "at://del/1" });
    const state = makeState({
      chatIndex: { "1704067200000-at://del/1": msg },
      chat: [msg],
    });
    const deleteMarker = makeMsg({ uri: "at://del/1", deleted: true } as any);
    const result = reduceChat(state, [deleteMarker], [], []);
    expect(result.chat).toHaveLength(0);
  });

  it("skips messages that are in pendingHides", () => {
    const msg = makeMsg({ text: "hidden", uri: "at://pending/1" });
    const state = makeState({ pendingHides: ["at://pending/1"] });
    const result = reduceChat(state, [msg], [], []);
    expect(result.chat).toHaveLength(0);
  });

  it("cleans up pendingHides after processing", () => {
    const msg = makeMsg({ text: "x", uri: "at://h/1" });
    const state = makeState({
      chatIndex: { "1704067200000-at://h/1": msg },
      chat: [msg],
      pendingHides: ["at://h/1"],
    });
    const result = reduceChat(state, [], [], ["at://h/1"]);
    expect(result.pendingHides).toHaveLength(0);
  });

  it("updates author profile when color changes", () => {
    const msg1 = makeMsg({ did: "did:plc:a", text: "hi" });
    const state = reduceChat(makeState(), [msg1], [], []);
    expect(state.authors["did:plc:a"]).toEqual({
      color: { red: 255, green: 255, blue: 255 },
    });

    const msg2 = makeMsg({
      did: "did:plc:a",
      text: "hi again",
      uri: "at://msg2",
      createdAt: "2024-01-01T00:00:01.000Z",
      chatProfile: { color: { red: 0, green: 0, blue: 0 } } as any,
    });
    const state2 = reduceChat(state, [msg2], [], []);
    expect(state2.authors["did:plc:a"]).toEqual({
      color: { red: 0, green: 0, blue: 0 },
    });
  });

  it("does not change author ref when color is the same", () => {
    const profile = { color: { red: 255, green: 255, blue: 255 } };
    const msg1 = makeMsg({
      did: "did:plc:a",
      text: "hi",
      chatProfile: profile as any,
    });
    const state = reduceChat(makeState(), [msg1], [], []);
    const authorsBefore = state.authors["did:plc:a"];

    const msg2 = makeMsg({
      did: "did:plc:a",
      text: "hi again",
      uri: "at://msg2",
      createdAt: "2024-01-01T00:00:01.000Z",
      chatProfile: profile as any,
    });
    const state2 = reduceChat(state, [msg2], [], []);
    // Same ref — no re-render trigger
    expect(state2.authors["did:plc:a"]).toBe(authorsBefore);
  });
});
