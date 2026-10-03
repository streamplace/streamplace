import { useEffect, useState } from "react";
import { type CaptionClock, presentedCaptionTime } from "./live-cues";

/** Freeze fallback caption presentation while playback is paused. */
export function useCaptionTime(
  clock: CaptionClock | null,
  paused: boolean,
): number | null {
  const [time, setTime] = useState<number | null>(null);
  useEffect(() => {
    if (paused) return;
    const update = () => setTime(presentedCaptionTime(clock, Date.now()));
    update();
    if (!clock) return;
    const timer = setInterval(update, 250);
    return () => clearInterval(timer);
  }, [clock, paused]);
  return time;
}
