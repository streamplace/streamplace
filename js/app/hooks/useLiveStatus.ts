import { useLivestreamStore, usePlayerStore } from "@streamplace/components";
import { useEffect, useState } from "react";

export function useLiveStatus() {
  const livestream = useLivestreamStore((state) => state.livestream);
  const segment = useLivestreamStore((state) => state.segment);
  const mode = usePlayerStore((state) => state.mode);
  const [now, setNow] = useState(Date.now);
  const latestActivity = Math.max(
    Date.parse(segment?.startTime ?? "") +
      Math.max(0, segment?.duration ?? 0) / 1e6 || -Infinity,
    Date.parse(livestream?.record.lastSeenAt ?? "") || -Infinity,
  );
  const liveUntil = latestActivity + 10_000;
  const isLive =
    mode === "live" &&
    livestream !== null &&
    !livestream.record.endedAt &&
    Number.isFinite(latestActivity) &&
    liveUntil > Math.max(now, Date.now());

  // Expire the signal even when the channel stops sending store updates.
  useEffect(() => {
    if (!isLive) return;
    const timeout = setTimeout(
      () => setNow(Date.now()),
      liveUntil - Date.now(),
    );
    return () => clearTimeout(timeout);
  }, [isLive, liveUntil]);

  return { livestream, segment, mode, isLive };
}
