import { afterEach, describe, expect, it, vi } from "vitest";
import { getLiveHLSUrl, getLiveLLHLSUrl } from "./streamplace-url";

describe("getLiveLLHLSUrl", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("builds the encoded LL-HLS main playlist URL", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => "https://example.com///",
    });

    expect(getLiveLLHLSUrl("did:key:z6Mk/live")).toBe(
      "https://example.com/api/playback/did%3Akey%3Az6Mk%2Flive/llhls/main.m3u8",
    );
  });
});

describe("getLiveHLSUrl", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("builds the encoded standard live playlist URL", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => "https://example.com///",
    });

    expect(getLiveHLSUrl("did:key:z6Mk/live")).toBe(
      "https://example.com/xrpc/place.stream.playback.getLivePlaylist?streamer=did%3Akey%3Az6Mk%2Flive",
    );
  });
});
