import { describe, expect, it } from "vitest";
import { resolveStreamAvatar } from "./resolve-stream-avatar";

describe("resolveStreamAvatar", () => {
  it("prefers the detailed DID profile avatar", () => {
    expect(
      resolveStreamAvatar({
        detailedAvatar: "https://cdn.example/detailed.jpg",
        profileAvatar: "https://cdn.example/profile.jpg",
        authorAvatar: "https://cdn.example/author.jpg",
      }),
    ).toBe("https://cdn.example/detailed.jpg");
  });

  it("falls back through the websocket profile and livestream author", () => {
    expect(
      resolveStreamAvatar({
        profileAvatar: "https://cdn.example/profile.jpg",
        authorAvatar: "https://cdn.example/author.jpg",
      }),
    ).toBe("https://cdn.example/profile.jpg");

    expect(
      resolveStreamAvatar({
        authorAvatar: "https://cdn.example/author.jpg",
      }),
    ).toBe("https://cdn.example/author.jpg");
  });

  it("returns undefined when no profile includes an avatar", () => {
    expect(resolveStreamAvatar({})).toBeUndefined();
  });
});
