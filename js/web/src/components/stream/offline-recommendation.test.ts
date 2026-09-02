import { describe, expect, it } from "vitest";
import { validateOfflineRecommendation } from "./offline-recommendation";

describe("validateOfflineRecommendation", () => {
  it("returns a recommendation only after its profile resolves", () => {
    expect(
      validateOfflineRecommendation(
        { did: "did:plc:recommended", source: "streamer" },
        { did: "did:plc:recommended", handle: "friend.example" },
        "did:plc:offline",
      ),
    ).toEqual({
      did: "did:plc:recommended",
      handle: "friend.example",
      source: "streamer",
    });
  });

  it("rejects unresolved and self recommendations", () => {
    expect(
      validateOfflineRecommendation(
        { did: "did:plc:recommended", source: "streamer" },
        undefined,
        "did:plc:offline",
      ),
    ).toBeNull();
    expect(
      validateOfflineRecommendation(
        { did: "did:plc:offline", source: "streamer" },
        { did: "did:plc:offline", handle: "offline.example" },
        "did:plc:offline",
      ),
    ).toBeNull();
  });
});
