import { describe, expect, it } from "vitest";
import { shouldShowUnmutePrompt } from "./player-controls";

describe("shouldShowUnmutePrompt", () => {
  it("shows while autoplay is playing silently", () => {
    expect(shouldShowUnmutePrompt(true, true)).toBe(true);
  });

  it("hides after the viewer unmutes", () => {
    expect(shouldShowUnmutePrompt(true, false)).toBe(false);
  });

  it("hides when playback is paused", () => {
    expect(shouldShowUnmutePrompt(false, true)).toBe(false);
  });
});
