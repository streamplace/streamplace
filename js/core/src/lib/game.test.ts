import { describe, expect, it } from "vitest";
import { getDidFromAtUri, getGameCoverUrl } from "./game";

describe("getDidFromAtUri", () => {
  it("extracts the DID from an at:// URI", () => {
    expect(getDidFromAtUri("at://did:plc:abc/place.stream.video/3kbp")).toBe(
      "did:plc:abc",
    );
  });

  it("returns an empty string for a malformed URI", () => {
    expect(getDidFromAtUri("nope")).toBe("");
  });
});

describe("getGameCoverUrl", () => {
  const media = (mediaType: string, cid?: string) =>
    ({
      mediaType,
      blob: cid ? { ref: { toString: () => cid } } : undefined,
    }) as any;

  it("prefers a cover over non-cover media", () => {
    const url = getGameCoverUrl(
      [media("screenshot", "cid-shot"), media("cover", "cid-cover")],
      "did:plc:abc",
    );
    expect(url).toBe(
      "https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:abc/cid-cover@jpeg",
    );
  });

  it("falls back to the first media item", () => {
    const url = getGameCoverUrl(
      [media("screenshot", "cid-shot")],
      "did:plc:abc",
    );
    expect(url).toContain("/cid-shot@jpeg");
  });

  it("returns undefined when media has no blob ref", () => {
    expect(getGameCoverUrl([media("cover")], "did:plc:abc")).toBeUndefined();
  });

  it("returns undefined for empty media", () => {
    expect(getGameCoverUrl(undefined, "did:plc:abc")).toBeUndefined();
  });
});
