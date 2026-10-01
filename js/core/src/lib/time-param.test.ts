import { describe, expect, it } from "vitest";
import { parseTimeParam } from "./time-param";

describe("parseTimeParam", () => {
  it("parses a bare number of seconds", () => {
    expect(parseTimeParam("151")).toBe(151);
    expect(parseTimeParam("0")).toBe(0);
    expect(parseTimeParam("90.5")).toBe(90.5);
    expect(parseTimeParam(42)).toBe(42);
  });

  it("parses h/m/s duration strings", () => {
    expect(parseTimeParam("1h2m3s")).toBe(3723);
    expect(parseTimeParam("2m30s")).toBe(150);
    expect(parseTimeParam("90s")).toBe(90);
    expect(parseTimeParam("1h")).toBe(3600);
    expect(parseTimeParam("5m")).toBe(300);
  });

  it("returns null for missing or malformed values", () => {
    expect(parseTimeParam(undefined)).toBeNull();
    expect(parseTimeParam(null)).toBeNull();
    expect(parseTimeParam("")).toBeNull();
    expect(parseTimeParam("   ")).toBeNull();
    expect(parseTimeParam("abc")).toBeNull();
    expect(parseTimeParam("12x")).toBeNull();
    expect(parseTimeParam("-5")).toBeNull();
    expect(parseTimeParam(-5)).toBeNull();
    expect(parseTimeParam("m")).toBeNull();
    expect(parseTimeParam("1m30")).toBeNull();
    expect(parseTimeParam(NaN)).toBeNull();
    expect(parseTimeParam(Infinity)).toBeNull();
    // Runtime-untyped inputs a URL can produce.
    expect(parseTimeParam(["1"])).toBeNull();
    expect(parseTimeParam({ t: 1 })).toBeNull();
    expect(parseTimeParam(true)).toBeNull();
  });
});
