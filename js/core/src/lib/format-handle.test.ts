import { describe, expect, it } from "vitest";
import { formatHandle } from "./format-handle";

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

  it.each([undefined, null, ""])(
    "returns bare did when handle is %j",
    (handle) => {
      expect(formatHandle({ handle, did: "did:plc:abc" }, "@")).toBe(
        "did:plc:abc",
      );
    },
  );
});
