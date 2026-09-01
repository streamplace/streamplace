import { mapClipError, useClipStore } from "./clip-store";

// The store constructs `new StreamplaceAgent(oauthSession)` and calls
// `agent.client.call(...)`; replace the module with a fake agent whose call is
// a controllable jest mock. The factory is self-contained (no hoisting
// pitfalls) and the mock is grabbed back via jest.requireMock.
jest.mock("streamplace", () => {
  const call = jest.fn();
  class FakeStreamplaceAgent {
    client = { call };
  }
  return {
    StreamplaceAgent: FakeStreamplaceAgent,
    place: {
      stream: {
        clip: {
          create: { $nsid: "place.stream.clip.create" },
          publish: { $nsid: "place.stream.clip.publish" },
          cancel: { $nsid: "place.stream.clip.cancel" },
        },
      },
    },
    __mockCall: call,
  };
});

const mockCall = (jest.requireMock("streamplace") as any)
  .__mockCall as jest.Mock;

const SESSION = { did: "did:plc:viewer" }; // stands in for a SessionManager
const STREAMER = "did:plc:streamer";
const LIVESTREAM = "at://did:plc:streamer/place.stream.livestream/abc";
const TTL_MS = 10 * 60 * 1000;

const createResult = () => ({
  clipId: "clip-1",
  previewUrl: "https://example.com/preview.mp4",
  expiresAt: new Date(Date.now() + TTL_MS).toISOString(),
  durationMs: 120000,
});

const publishResult = {
  videoUri: "at://did:plc:viewer/place.stream.video/clip1",
  clipUri: "at://did:plc:viewer/place.stream.clip.entry/clip1",
};

beforeEach(() => {
  jest.useFakeTimers();
  jest.setSystemTime(new Date("2026-01-01T00:00:00Z"));
  useClipStore.getState().discard();
  mockCall.mockReset();
  mockCall.mockImplementation(async (proc: { $nsid: string }) => {
    if (proc.$nsid === "place.stream.clip.create") return createResult();
    if (proc.$nsid === "place.stream.clip.publish") return { ...publishResult };
    if (proc.$nsid === "place.stream.clip.cancel") return { cancelled: true };
    throw new Error("unexpected xrpc call");
  });
});

const requestClip = () =>
  useClipStore.getState().requestClip({
    streamerDID: STREAMER,
    oauthSession: SESSION as any,
    livestreamUri: LIVESTREAM,
  });

describe("requestClip", () => {
  it("errors with 'Still loading your session' while the session loads", async () => {
    await useClipStore.getState().requestClip({
      streamerDID: STREAMER,
      oauthSession: undefined,
      livestreamUri: LIVESTREAM,
    });
    const s = useClipStore.getState();
    expect(s.status).toBe("error");
    expect(s.error).toBe("Still loading your session");
    expect(mockCall).not.toHaveBeenCalled();
  });

  it("errors with 'Please sign in to clip' when logged out, without a network call", async () => {
    await useClipStore.getState().requestClip({
      streamerDID: STREAMER,
      oauthSession: null,
      livestreamUri: LIVESTREAM,
    });
    const s = useClipStore.getState();
    expect(s.status).toBe("error");
    expect(s.error).toBe("Please sign in to clip");
    expect(mockCall).not.toHaveBeenCalled();
  });

  it("requests a 120s grab and enters editing with a full-duration trim", async () => {
    await requestClip();
    const s = useClipStore.getState();
    expect(s.status).toBe("editing");
    expect(s.clipId).toBe("clip-1");
    expect(s.previewUrl).toBe("https://example.com/preview.mp4");
    expect(s.durationMs).toBe(120000);
    expect(s.trimStart).toBe(0);
    expect(s.trimEnd).toBe(120000);
    expect(s.livestreamUri).toBe(LIVESTREAM);
    expect(s.timeRemaining).toBe(TTL_MS);
    expect(mockCall).toHaveBeenCalledWith(
      expect.objectContaining({ $nsid: "place.stream.clip.create" }),
      { streamer: STREAMER, durationMs: 120000 },
    );
  });

  it.each([
    ["ClippingDisabled", "The streamer has disabled clipping"],
    ["RateLimited", "You're clipping too fast, try again in a moment"],
    ["NotLive", "The stream isn't live right now"],
    ["NoContent", "Nothing to clip yet, try again in a moment"],
    ["Unauthorized", "Please sign in to clip"],
  ])("maps the %s create error to user-facing copy", async (name, copy) => {
    mockCall.mockRejectedValueOnce({ error: name, message: "server says" });
    await requestClip();
    const s = useClipStore.getState();
    expect(s.status).toBe("error");
    expect(s.error).toBe(copy);
    expect(s.clipId).toBeNull();
  });

  it("falls back to the server message for unknown create errors", async () => {
    mockCall.mockRejectedValueOnce({
      error: "MysteryError",
      message: "mystery",
    });
    await requestClip();
    expect(useClipStore.getState().error).toBe("mystery");
  });
});

describe("setTrim", () => {
  beforeEach(requestClip);

  it("clamps to the track bounds", () => {
    useClipStore.getState().setTrim(-1000, 130000);
    const s = useClipStore.getState();
    expect(s.trimStart).toBe(0);
    expect(s.trimEnd).toBe(120000);
  });

  it("enforces the 5s minimum by pushing the far boundary", () => {
    useClipStore.getState().setTrim(18000, 20000);
    const s = useClipStore.getState();
    expect(s.trimStart).toBe(18000);
    expect(s.trimEnd).toBe(23000);
  });

  it("stores integer trims even when given gesture-derived floats", () => {
    // px→ms conversion in the timeline produces non-integer ms; publish sends
    // start/end as int64 so the store must round.
    useClipStore.getState().setTrim(49830.50847457627, 90000.1234);
    const s = useClipStore.getState();
    expect(Number.isInteger(s.trimStart)).toBe(true);
    expect(Number.isInteger(s.trimEnd)).toBe(true);
    expect(s.trimStart).toBe(49831);
    expect(s.trimEnd).toBe(90000);
  });
});

describe("publish", () => {
  it("does not publish without a title", async () => {
    await requestClip();
    await useClipStore.getState().publish();
    const s = useClipStore.getState();
    expect(s.status).toBe("editing");
    expect(s.error).toBe("Title is required");
    expect(mockCall).not.toHaveBeenCalledWith(
      expect.objectContaining({ $nsid: "place.stream.clip.publish" }),
      expect.anything(),
    );
  });

  it("publishes the trim window and title", async () => {
    await requestClip();
    useClipStore.getState().setTrim(30000, 90000);
    useClipStore.getState().setTitle("Great moment");
    await useClipStore.getState().publish();
    const s = useClipStore.getState();
    expect(s.status).toBe("published");
    expect(s.videoUri).toBe(publishResult.videoUri);
    expect(s.clipUri).toBe(publishResult.clipUri);
    expect(mockCall).toHaveBeenCalledWith(
      expect.objectContaining({ $nsid: "place.stream.clip.publish" }),
      {
        clipId: "clip-1",
        livestream: LIVESTREAM,
        start: 30000,
        end: 90000,
        title: "Great moment",
      },
    );
  });

  it.each([
    ["DraftNotFound", "This clip draft no longer exists, start over"],
    ["DraftExpired", "This clip draft expired, start over"],
    ["Unauthorized", "Please sign in to publish"],
  ])(
    "maps the %s publish error to copy and keeps the draft for retry",
    async (name, copy) => {
      await requestClip();
      useClipStore.getState().setTitle("x");
      mockCall.mockRejectedValueOnce({ error: name, message: "server says" });
      await useClipStore.getState().publish();
      const s = useClipStore.getState();
      expect(s.status).toBe("error");
      expect(s.error).toBe(copy);
      // clipId retained → the editor stays open with a retryable Publish.
      expect(s.clipId).toBe("clip-1");
    },
  );

  it("refuses to publish an expired draft without sending a request", async () => {
    await requestClip();
    useClipStore.getState().setTitle("x");
    // Advance past the TTL; the tick flips editing → expired.
    useClipStore.getState().tick(Date.now() + TTL_MS + 1000);
    expect(useClipStore.getState().status).toBe("expired");
    await useClipStore.getState().publish();
    expect(useClipStore.getState().status).toBe("expired");
    expect(mockCall).not.toHaveBeenCalledWith(
      expect.objectContaining({ $nsid: "place.stream.clip.publish" }),
      expect.anything(),
    );
  });

  it("does not resurrect a draft discarded while publishing", async () => {
    await requestClip();
    useClipStore.getState().setTitle("x");
    let resolvePublish!: (v: unknown) => void;
    mockCall.mockImplementationOnce(
      (proc: { $nsid: string }) =>
        new Promise((resolve) => {
          resolvePublish = resolve;
          void proc;
        }),
    );
    const inFlight = useClipStore.getState().publish();
    // The dialog X/backdrop discards while the request is still in flight.
    useClipStore.getState().discard();
    resolvePublish({ ...publishResult });
    await inFlight;
    expect(useClipStore.getState().status).toBe("idle");
    expect(useClipStore.getState().videoUri).toBeNull();
  });
});

describe("cancel", () => {
  it("deletes the draft server-side and resets to idle", async () => {
    await requestClip();
    await useClipStore.getState().cancel();
    const s = useClipStore.getState();
    expect(s.status).toBe("idle");
    expect(s.clipId).toBeNull();
    expect(mockCall).toHaveBeenCalledWith(
      expect.objectContaining({ $nsid: "place.stream.clip.cancel" }),
      { clipId: "clip-1" },
    );
  });

  it("is best-effort: a server error still resets to idle", async () => {
    await requestClip();
    mockCall.mockRejectedValueOnce({ error: "DraftNotFound" });
    await useClipStore.getState().cancel();
    expect(useClipStore.getState().status).toBe("idle");
  });

  it("does not send a cancel request after publishing", async () => {
    await requestClip();
    useClipStore.getState().setTitle("x");
    await useClipStore.getState().publish();
    mockCall.mockClear();
    await useClipStore.getState().cancel();
    expect(useClipStore.getState().status).toBe("idle");
    expect(mockCall).not.toHaveBeenCalledWith(
      expect.objectContaining({ $nsid: "place.stream.clip.cancel" }),
      expect.anything(),
    );
  });

  it("does nothing with no draft", async () => {
    await useClipStore.getState().cancel();
    expect(mockCall).not.toHaveBeenCalled();
  });
});

describe("expiry", () => {
  it("ticks down timeRemaining while editing and flips to expired at zero", async () => {
    await requestClip();
    useClipStore.getState().tick(Date.now() + 60_000);
    const s = useClipStore.getState();
    expect(s.status).toBe("editing");
    expect(s.timeRemaining).toBe(TTL_MS - 60_000);

    useClipStore.getState().tick(Date.now() + TTL_MS + 1000);
    const s2 = useClipStore.getState();
    expect(s2.status).toBe("expired");
    expect(s2.timeRemaining).toBe(0);
  });
});

describe("module singleton", () => {
  it("keeps draft state across module re-imports (survives navigation)", async () => {
    await requestClip();
    useClipStore.getState().setTitle("persisted title");
    // Re-importing resolves to the same cached module instance, so the draft
    // survives whatever unmounts/remounts the editor.
    const again = require("./clip-store");
    expect(again.useClipStore).toBe(useClipStore);
    expect(again.useClipStore.getState().title).toBe("persisted title");
    expect(again.useClipStore.getState().status).toBe("editing");
  });
});

describe("mapClipError", () => {
  it("maps each create error name", () => {
    expect(mapClipError("create", { error: "ClippingDisabled" })).toBe(
      "The streamer has disabled clipping",
    );
    expect(mapClipError("create", { error: "RateLimited" })).toBe(
      "You're clipping too fast, try again in a moment",
    );
  });

  it("maps each publish error name", () => {
    expect(mapClipError("publish", { error: "DraftExpired" })).toBe(
      "This clip draft expired, start over",
    );
    expect(mapClipError("publish", { error: "Unauthorized" })).toBe(
      "Please sign in to publish",
    );
  });

  it("falls back to the server message and then a generic string", () => {
    expect(
      mapClipError("create", { error: "Unknown", message: "custom" }),
    ).toBe("custom");
    expect(mapClipError("create", new Error("plain error"))).toBe(
      "plain error",
    );
    expect(mapClipError("create", {})).toBe("Failed to create clip");
    expect(mapClipError("publish", {})).toBe("Failed to publish clip");
  });
});
