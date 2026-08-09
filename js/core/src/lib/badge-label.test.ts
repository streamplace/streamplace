import { describe, expect, it } from "vitest";
import { formatBadgeIssuer, formatBadgeLabel } from "./badge-label";

describe("formatBadgeLabel", () => {
  it("prefixes a named VIP badge with its badge type", () => {
    expect(
      formatBadgeLabel({
        badgeType: "place.stream.badge.defs#vip",
        badgeName: "  Founding Supporter  ",
        fallbackLabel: "VIP",
        vipLabel: "VIP",
      }),
    ).toBe("(VIP) Founding Supporter");
  });

  it("keeps the fallback label for an unnamed VIP badge", () => {
    expect(
      formatBadgeLabel({
        badgeType: "place.stream.badge.defs#vip",
        badgeName: "  ",
        fallbackLabel: "VIP",
        vipLabel: "VIP",
      }),
    ).toBe("VIP");
  });

  it("preserves the existing label for other badge types", () => {
    expect(
      formatBadgeLabel({
        badgeType: "place.stream.badge.defs#event",
        badgeName: "Summer 2026",
        fallbackLabel: "Summer 2026",
        vipLabel: "VIP",
      }),
    ).toBe("Summer 2026");
  });
});

describe("formatBadgeIssuer", () => {
  it("shows a resolved handle instead of its DID", () => {
    expect(
      formatBadgeIssuer("did:plc:abcdefghijklmnop", "badge-maker.test"),
    ).toBe("@badge-maker.test");
  });

  it("does not duplicate an existing handle prefix", () => {
    expect(formatBadgeIssuer("did:plc:abcdefghijklmnop", "@issuer.test")).toBe(
      "@issuer.test",
    );
  });

  it("shortens the DID while its profile is unavailable", () => {
    expect(formatBadgeIssuer("did:plc:abcdefghijklmnop")).toBe("did:plc:abcd…");
  });
});
