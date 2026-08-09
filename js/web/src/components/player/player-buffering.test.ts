import { describe, expect, it } from "vitest";
import {
  getBufferingOverlayPresentation,
  getBufferingState,
  shouldShowBufferingIndicator,
} from "./player-buffering";

describe("getBufferingState", () => {
  it.each(["loadstart", "waiting", "seeking"] as const)(
    "starts buffering on %s",
    (event) => {
      expect(getBufferingState(event)).toBe(true);
    },
  );

  it("does not report buffering when downloads stall but playback can continue", () => {
    expect(getBufferingState("stalled")).toBe(false);
  });

  it.each(["canplay", "playing", "seeked", "pause", "ended", "error"] as const)(
    "stops buffering on %s",
    (event) => {
      expect(getBufferingState(event)).toBe(false);
    },
  );
});

describe("shouldShowBufferingIndicator", () => {
  it("shows while active playback is buffering", () => {
    expect(
      shouldShowBufferingIndicator({
        active: true,
        buffering: true,
        bigPlay: false,
        hasError: false,
      }),
    ).toBe(true);
  });

  it.each([
    { active: false, buffering: true, bigPlay: false, hasError: false },
    { active: true, buffering: false, bigPlay: false, hasError: false },
    { active: true, buffering: true, bigPlay: true, hasError: false },
    { active: true, buffering: true, bigPlay: false, hasError: true },
  ])("hides when playback feedback has higher priority", (state) => {
    expect(shouldShowBufferingIndicator(state)).toBe(false);
  });
});

describe("getBufferingOverlayPresentation", () => {
  it("exposes the loading status while fading in", () => {
    expect(getBufferingOverlayPresentation(true)).toEqual({
      ariaHidden: false,
      opacityClass: "opacity-100",
    });
  });

  it("removes the loading status from the accessibility tree while fading out", () => {
    expect(getBufferingOverlayPresentation(false)).toEqual({
      ariaHidden: true,
      opacityClass: "opacity-0",
    });
  });
});
