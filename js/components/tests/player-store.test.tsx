import { describe, expect, it, vi } from "vitest";

vi.mock("../src/livestream-store", () => ({
  useLivestreamStore: () => ({}),
}));
vi.mock("../src/streamplace-store", () => ({
  useStreamplaceStore: () => ({}),
}));
vi.mock("../src/storage", () => ({
  default: { getItem: async () => null, setItem: async () => {} },
}));

import { makePlayerStore } from "../src/player-store/player-store";

describe("VOD playback controls", () => {
  it("clamps playback speed and resets it when switching to live", () => {
    const store = makePlayerStore();
    const { setMode, setPlaybackRate } = store.getState();

    expect(store.getState().playbackRate).toBe(1);

    setMode("vod");
    setPlaybackRate(1.25);
    expect(store.getState().playbackRate).toBe(1.25);

    setPlaybackRate(3);
    expect(store.getState().playbackRate).toBe(2);

    setMode("live");
    setPlaybackRate(0.5);
    expect(store.getState().playbackRate).toBe(1);
  });

  it("keeps only the latest player feedback action", () => {
    const store = makePlayerStore();
    const { showFeedback } = store.getState();

    showFeedback({ type: "speed", playbackRate: 1.25 });
    showFeedback({ type: "frame", direction: "forward" });

    expect(store.getState().feedback).toEqual({
      type: "frame",
      direction: "forward",
    });
  });
});
