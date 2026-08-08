import { describe, expect, it } from "vitest";
import { formatViewers } from "./format";

describe("formatViewers", () => {
  it("returns null for null input", () => {
    expect(formatViewers(null)).toBeNull();
  });

  it("returns null for undefined input", () => {
    expect(formatViewers(undefined as unknown as null)).toBeNull();
  });

  it("formats numbers below 1000 as locale strings", () => {
    expect(formatViewers(42)).toBe("42");
    expect(formatViewers(999)).toBe("999");
  });

  it("formats thousands with K suffix", () => {
    expect(formatViewers(1000)).toBe("1.0K");
    expect(formatViewers(1500)).toBe("1.5K");
    expect(formatViewers(999999)).toBe("1000.0K");
  });

  it("formats millions with M suffix", () => {
    expect(formatViewers(1_000_000)).toBe("1.0M");
    expect(formatViewers(2_500_000)).toBe("2.5M");
  });

  it("formats zero", () => {
    expect(formatViewers(0)).toBe("0");
  });
});
