import { describe, expect, it } from "vitest";
import { validateOfflineRecommendation } from "./offline-recommendation";

describe("validateOfflineRecommendation", () => {
  it("keeps a resolved recommendation without a verified handle", () => {
    expect(
      validateOfflineRecommendation(
        { did: "did:plc:recommended", source: "streamer" },
        { did: "did:plc:recommended", handle: "" },
        "did:plc:offline",
      ),
    ).toEqual({
      did: "did:plc:recommended",
      handle: "",
      source: "streamer",
    });
  });

  it("rejects unresolved, mismatched, and self recommendations", () => {
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
    expect(
      validateOfflineRecommendation(
        { did: "did:plc:recommended", source: "streamer" },
        { did: "did:plc:other", handle: "other.example" },
        "did:plc:offline",
      ),
    ).toBeNull();
  });
});
