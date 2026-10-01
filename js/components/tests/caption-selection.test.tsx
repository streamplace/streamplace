import { activeLiveCaptions } from "@streamplace/core";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { useCaptionSelection } from "../src/components/captions/use-captions";

const boundary = vi.hoisted(() => ({
  player: {
    mode: "live",
    status: "start",
    captionTrackId: null,
    captionServerTracks: [
      { id: "human", language: "en", source: "human", origin: "canonical" },
    ],
    captionElementTracks: [],
  },
  live: {
    captionTracks: [],
    captionClock: { startMs: 10000, receivedAt: 1000 },
    liveCaptions: {
      speech: {
        id: "speech",
        trackId: "human",
        text: "Live speech",
        final: true,
        startMs: 11000,
        endMs: 20000,
        updatedAt: 1000,
      },
    },
  },
}));
vi.mock("../src/player-store", async () => ({
  // Vitest hoists this factory before imports; load only the enum module here.
  PlayerStatus: (await import("../src/player-store/player-state")).PlayerStatus,
  usePlayerStore: <T,>(selector: (state: typeof boundary.player) => T) =>
    selector(boundary.player),
}));
vi.mock("../src/livestream-store", () => ({
  useLivestreamStoreOptional: <T,>(
    selector: (state: typeof boundary.live) => T,
  ) => selector(boundary.live),
}));
vi.mock("../src/streamplace-store", () => ({
  useCaptionLanguage: () => "en",
  useCaptionsEnabled: () => true,
}));

it("advances live captions during non-pause statuses and freezes only an explicit pause", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  vi.setSystemTime(1000);
  const container = document.createElement("div");
  const root = createRoot(container);
  function Caption() {
    const { active, presented } = useCaptionSelection();
    return (
      <span>
        {active && presented !== null
          ? activeLiveCaptions(boundary.live.liveCaptions, active.id, presented)
              .map((cue) => cue.text)
              .join("\n")
          : ""}
      </span>
    );
  }
  try {
    await act(async () => root.render(<Caption />));
    expect(container.textContent).toBe("");
    for (const status of ["start", "waiting", "stalled", "suspend", "mute"]) {
      boundary.player.status = status;
      await act(async () => root.render(<Caption />));
      await act(async () => vi.advanceTimersByTime(1000));
      expect(container.textContent, status).toBe("Live speech");
    }
    boundary.player.status = "pause";
    await act(async () => root.render(<Caption />));
    boundary.live.captionClock = { startMs: 25000, receivedAt: Date.now() };
    await act(async () => root.render(<Caption />));
    await act(async () => vi.advanceTimersByTime(5000));
    expect(container.textContent).toBe("Live speech");
    boundary.player.status = "playing";
    await act(async () => root.render(<Caption />));
    expect(container.textContent).toBe("");
  } finally {
    await act(async () => root.unmount());
    vi.useRealTimers();
    vi.unstubAllGlobals();
  }
});
