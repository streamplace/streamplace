import { useEffect, useRef, useState } from "react";
import { useLivestreamStore } from "../livestream-store";

export type ConnectionQuality = "good" | "degraded" | "poor";

function getLiveConnectionQuality(
  timeBetweenSegments: number | null,
  range: number | null,
  numOfSegments: number = 1,
): ConnectionQuality {
  if (timeBetweenSegments === null || range === null) return "poor";

  if (timeBetweenSegments <= 1500 && range <= (1500 * 60) / numOfSegments) {
    return "good";
  }
  if (timeBetweenSegments <= 3000 && range <= (3000 * 60) / numOfSegments) {
    return "degraded";
  }
  return "poor";
}

export function useSegmentTiming() {
  const latestSegment = useLivestreamStore((x) => x.segment);
  const [segmentDeltas, setSegmentDeltas] = useState<number[]>([]);
  const prevSegmentRef = useRef<any>();
  const prevTimestampRef = useRef<number | null>(null);

  // Dummy state to force update every second
  const [, setNow] = useState(Date.now());

  useEffect(() => {
    const interval = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (latestSegment && prevSegmentRef.current !== latestSegment) {
      const now = Date.now();
      if (prevTimestampRef.current !== null) {
        const delta = now - prevTimestampRef.current;
        // Only store the last 25 deltas
        setSegmentDeltas((prev) => [...prev, delta].slice(-25));
      }
      prevTimestampRef.current = now;
      prevSegmentRef.current = latestSegment;
    }
  }, [latestSegment]);

  // The most recent time between segments
  const timeBetweenSegments =
    segmentDeltas.length > 0
      ? segmentDeltas[segmentDeltas.length - 1]
      : prevTimestampRef.current
        ? Date.now() - prevTimestampRef.current
        : null;

  // Calculate mean and range of deltas
  const mean =
    segmentDeltas.length > 0
      ? Math.round(
          segmentDeltas.reduce((acc, curr) => acc + curr, 0) /
            segmentDeltas.length,
        )
      : null;

  const range =
    segmentDeltas.length > 0
      ? Math.max(...segmentDeltas) - Math.min(...segmentDeltas)
      : null;

  let to_ret = {
    segmentDeltas,
    timeBetweenSegments,
    mean,
    range,
    connectionQuality: "poor",
  };

  to_ret.connectionQuality = getLiveConnectionQuality(
    timeBetweenSegments,
    range,
    segmentDeltas.length,
  );

  return to_ret;
}
