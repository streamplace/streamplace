import {
  activeLiveCaptions,
  type CaptionClock,
  useCaptionTime,
} from "@streamplace/core";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";

it("holds fallback speech through pause and new segment announcements, then resumes", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  vi.setSystemTime(1000);
  const container = document.createElement("div");
  const root = createRoot(container);
  const cues = {
    speech: {
      id: "speech",
      trackId: "canonical",
      text: "Speech",
      final: true,
      startMs: 10000,
      endMs: 12000,
      shiftMs: 0,
      updatedAt: 1000,
    },
  };
  function Caption({
    clock,
    paused,
  }: {
    clock: CaptionClock;
    paused: boolean;
  }) {
    const time = useCaptionTime(clock, paused);
    return (
      <span>
        {time === null
          ? ""
          : activeLiveCaptions(cues, "canonical", time)
              .map((cue) => cue.text)
              .join("\n")}
      </span>
    );
  }
  try {
    const clock = { startMs: 10000, receivedAt: 1000 };
    await act(async () =>
      root.render(<Caption clock={clock} paused={false} />),
    );
    expect(container.textContent).toBe("Speech");
    await act(async () => vi.advanceTimersByTime(500));
    await act(async () => root.render(<Caption clock={clock} paused />));
    await act(async () => vi.advanceTimersByTime(5000));
    const next = { startMs: 15000, receivedAt: Date.now() };
    await act(async () => root.render(<Caption clock={next} paused />));
    expect(container.textContent).toBe("Speech");
    await act(async () => root.render(<Caption clock={next} paused={false} />));
    expect(container.textContent).toBe("");
  } finally {
    await act(async () => root.unmount());
    vi.useRealTimers();
    vi.unstubAllGlobals();
  }
});
