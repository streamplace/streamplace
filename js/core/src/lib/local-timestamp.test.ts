import { describe, expect, it, vi } from "vitest";
import { formatLocalTimestamp } from "./local-timestamp";

describe("formatLocalTimestamp", () => {
  it.each([
    [480, "2026-01-01T16:30:45.123-08:00"],
    [420, "2026-01-01T17:30:45.123-07:00"],
    [-345, "2026-01-02T06:15:45.123+05:45"],
    [210, "2026-01-01T21:00:45.123-03:30"],
    [0, "2026-01-02T00:30:45.123+00:00"],
  ])("preserves the instant with device offset %i", (offset, expected) => {
    const date = new Date("2026-01-02T00:30:45.123Z");
    // The OS supplies the offset; keep the test independent of the host zone.
    vi.spyOn(date, "getTimezoneOffset").mockReturnValue(offset);
    const timestamp = formatLocalTimestamp(date);
    expect(timestamp).toBe(expected);
    expect(Date.parse(timestamp)).toBe(date.getTime());
    expect(date.toISOString()).toBe("2026-01-02T00:30:45.123Z");
  });
});
