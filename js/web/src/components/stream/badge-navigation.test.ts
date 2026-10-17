import { describe, expect, it } from "vitest";
import { getAdjacentBadgeIndex } from "./badge-navigation";

describe("getAdjacentBadgeIndex", () => {
  it("moves between badges in either direction", () => {
    expect(getAdjacentBadgeIndex(1, 3, 1)).toBe(2);
    expect(getAdjacentBadgeIndex(1, 3, -1)).toBe(0);
  });

  it("wraps at both ends", () => {
    expect(getAdjacentBadgeIndex(2, 3, 1)).toBe(0);
    expect(getAdjacentBadgeIndex(0, 3, -1)).toBe(2);
  });

  it("keeps a single badge selected", () => {
    expect(getAdjacentBadgeIndex(0, 1, 1)).toBe(0);
    expect(getAdjacentBadgeIndex(0, 1, -1)).toBe(0);
  });

  it("returns null when there are no badges", () => {
    expect(getAdjacentBadgeIndex(0, 0, 1)).toBeNull();
  });
});
