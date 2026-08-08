import { describe, expect, it } from "vitest";
import { formatHandle, formatHandleWithAt } from "./format-handle";

describe("formatHandle", () => {
  it("returns handle when valid", () => {
    expect(
      formatHandle({ handle: "alice.bsky.social", did: "did:plc:abc" }),
    ).toBe("alice.bsky.social");
  });

  it("returns did when handle is invalid", () => {
    expect(formatHandle({ handle: "handle.invalid", did: "did:plc:abc" })).toBe(
      "did:plc:abc",
    );
  });

  it("prepends prefix when given", () => {
    expect(
      formatHandle({ handle: "alice.bsky.social", did: "did:plc:abc" }, "@"),
    ).toBe("@alice.bsky.social");
  });

  it("does not prepend prefix when handle is invalid", () => {
    expect(
      formatHandle({ handle: "handle.invalid", did: "did:plc:abc" }, "@"),
    ).toBe("did:plc:abc");
  });
});

describe("formatHandleWithAt", () => {
  it("returns @handle for valid handle", () => {
    expect(
      formatHandleWithAt({ handle: "bob.bsky.social", did: "did:plc:def" }),
    ).toBe("@bob.bsky.social");
  });

  it("returns did for invalid handle", () => {
    expect(
      formatHandleWithAt({ handle: "handle.invalid", did: "did:plc:def" }),
    ).toBe("did:plc:def");
  });
});
