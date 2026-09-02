import { describe, expect, it } from "vitest";
import {
  buildVodShareLinks,
  getAuthorHandle,
  getOptimisticLikeState,
  shouldCollapseDescription,
} from "./vod-watch";

describe("getAuthorHandle", () => {
  it("uses the handle as the creator's only identifier", () => {
    expect(
      getAuthorHandle(
        { handle: "chicago.at", displayName: "chicago.at" },
        "chicago.at",
      ),
    ).toBe("chicago.at");
  });

  it("ignores a distinct display name", () => {
    expect(
      getAuthorHandle(
        { handle: "chicago.at", displayName: "Chicago" },
        "did:plc:chicago",
      ),
    ).toBe("chicago.at");
  });

  it("falls back to the route identifier while profile data loads", () => {
    expect(getAuthorHandle({ displayName: "Chicago" }, "chicago.at")).toBe(
      "chicago.at",
    );
  });
});

describe("buildVodShareLinks", () => {
  it("builds canonical, embed, and iframe targets", () => {
    expect(
      buildVodShareLinks(
        "https://stream.place/ignored/path",
        "chicago.at",
        "3msefzx6st7m4",
      ),
    ).toEqual({
      pageUrl: "https://stream.place/chicago.at/video/3msefzx6st7m4",
      embedUrl: "https://stream.place/embed/chicago.at/video/3msefzx6st7m4",
      embedCode:
        '<iframe src="https://stream.place/embed/chicago.at/video/3msefzx6st7m4" width="640" height="360" frameborder="0" allowfullscreen></iframe>',
    });
  });

  it("encodes route segments in shared URLs", () => {
    const links = buildVodShareLinks(
      "https://stream.place",
      "name/with/slash",
      "tid with space",
    );

    expect(links.pageUrl).toBe(
      "https://stream.place/name%2Fwith%2Fslash/video/tid%20with%20space",
    );
  });
});

describe("getOptimisticLikeState", () => {
  it("increments when liking", () => {
    expect(getOptimisticLikeState({ liked: false, count: 4 })).toEqual({
      liked: true,
      count: 5,
    });
  });

  it("never decrements below zero when unliking", () => {
    expect(getOptimisticLikeState({ liked: true, count: 0 })).toEqual({
      liked: false,
      count: 0,
    });
  });
});

describe("shouldCollapseDescription", () => {
  it("keeps short descriptions fully visible", () => {
    expect(shouldCollapseDescription("A short description.")).toBe(false);
  });

  it("collapses descriptions beyond the readable preview", () => {
    expect(shouldCollapseDescription("x".repeat(181))).toBe(true);
  });
});
