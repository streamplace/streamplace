import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { useDashboardStore } from "./dashboard-store-context";

/** How often we sample the live metrics into the history buffer. */
const SAMPLE_INTERVAL_MS = 2000;

/** Range presets — sliding-window lengths. */
export type DashboardRange = "1m" | "2m" | "5m" | "10m" | "25m";

export const DASHBOARD_RANGES: {
  id: DashboardRange;
  label: string;
  samples: number;
}[] = [
  { id: "1m", label: "1m", samples: 30 },
  { id: "2m", label: "2m", samples: 60 },
  { id: "5m", label: "5m", samples: 150 },
  { id: "10m", label: "10m", samples: 300 },
  { id: "25m", label: "25m", samples: 750 },
];

/** One sample of the metric history. */
export interface DashboardMetricsPoint {
  t: number;
  bitrate: number;
  viewers: number;
  segmentTiming: number;
  chatRate: number;
  fps: number;
}

function emptyPoint(t: number): DashboardMetricsPoint {
  return { t, bitrate: 0, viewers: 0, segmentTiming: 0, chatRate: 0, fps: 0 };
}

export interface DashboardMetricsValue {
  history: DashboardMetricsPoint[];
  range: DashboardRange;
  setRange: (r: DashboardRange) => void;
  /** True when there's a live stream. Used for the connection-quality
   *  indicator and to distinguish "pre-live" from a real bad reading. */
  hasLivestream: boolean;
}

const DashboardMetricsContext = createContext<DashboardMetricsValue | null>(
  null,
);

/**
 * Hook for consuming the dashboard's time-series metrics. Must be used
 * inside `<DashboardMetricsProvider>`.
 */
export function useDashboardMetrics(): DashboardMetricsValue {
  const ctx = useContext(DashboardMetricsContext);
  if (!ctx) {
    throw new Error(
      "useDashboardMetrics must be used inside <DashboardMetricsProvider>",
    );
  }
  return ctx;
}

interface ProviderProps {
  children: ReactNode;
}

/**
 * Continuously samples the live stream's metrics into a rolling-window
 * history buffer. Mount this high in the tree (the dashboard chrome) so
 * the buffer keeps populating while the user navigates between dashboard
 * sub-routes. When the user is on a non-dashboard page the provider
 * unmounts and tracking stops — the buffer is fresh on remount.
 */
export function DashboardMetricsProvider({ children }: ProviderProps) {
  const store = useDashboardStore();
  const [range, setRange] = useState<DashboardRange>("2m");
  const historyLength =
    DASHBOARD_RANGES.find((r) => r.id === range)?.samples ??
    DASHBOARD_RANGES[0].samples;

  const [history, setHistory] = useState<DashboardMetricsPoint[]>(() =>
    Array.from({ length: historyLength }, () => emptyPoint(Date.now())),
  );

  // Refs for the live values that we sample into history on each tick.
  // Keeping these out of state avoids re-rendering on every segment update.
  const currentBitrateRef = useRef(0);
  const currentSegmentTimingRef = useRef(0);
  const currentFpsRef = useRef(0);
  const latestInputsRef = useRef({ chat: [] as any[], viewers: 0 });
  latestInputsRef.current = {
    chat: store.getState().chat,
    viewers: store.getState().viewers ?? 0,
  };

  // Subscribe to the store so the refs stay current. We don't render off
  // of these — they're read inside the sampling interval.
  const storeState = useStore(
    store,
    useShallow((s) => ({
      segment: s.segment,
      livestream: s.livestream,
      chat: s.chat,
      viewers: s.viewers,
    })),
  );

  // Update bitrate from the current segment.
  useEffect(() => {
    const seg = store.getState().segment;
    if (!seg?.size || !seg?.duration) return;
    const kbps = (seg.size * 8) / (seg.duration / 1_000_000_000) / 1000;
    currentBitrateRef.current = kbps;
  }, [
    store,
    store.getState().segment?.size,
    store.getState().segment?.duration,
  ]);

  // Update FPS from the current segment's video track.
  useEffect(() => {
    const seg = store.getState().segment;
    const videoTrack = seg?.video?.[0];
    if (!videoTrack?.framerate) return;
    const { num, den } = videoTrack.framerate;
    if (!den) return;
    currentFpsRef.current = num / den;
  }, [store, store.getState().segment?.video]);

  // Update segment timing (ms since previous segment).
  const lastSegmentAtRef = useRef<number | null>(null);
  useEffect(() => {
    const seg = store.getState().segment;
    if (!seg) return;
    const now = Date.now();
    if (lastSegmentAtRef.current !== null) {
      currentSegmentTimingRef.current = now - lastSegmentAtRef.current;
    }
    lastSegmentAtRef.current = now;
  }, [store, store.getState().segment]);

  // When the user changes the range, resize the history buffer. The window
  // then refills as new samples arrive.
  useEffect(() => {
    setHistory(
      Array.from({ length: historyLength }, () => emptyPoint(Date.now())),
    );
  }, [historyLength]);

  // Sample on a fixed interval. Captures the current bitrate, segment
  // timing, FPS, viewers, and chat rate into the history ring buffer.
  useEffect(() => {
    const id = setInterval(() => {
      const now = Date.now();
      const state = store.getState();
      const chat = state.chat ?? [];
      const viewers = state.viewers ?? 0;
      const oneMinuteAgo = now - 60_000;
      const chatRate = chat.filter((msg: any) => {
        try {
          return new Date(msg.indexedAt).getTime() > oneMinuteAgo;
        } catch {
          return false;
        }
      }).length;

      setHistory((prev) => {
        const next = prev.slice(1);
        next.push({
          t: now,
          bitrate: currentBitrateRef.current,
          viewers,
          segmentTiming: currentSegmentTimingRef.current,
          chatRate,
          fps: currentFpsRef.current,
        });
        return next;
      });
    }, SAMPLE_INTERVAL_MS);
    return () => clearInterval(id);
  }, [store]);

  const value = useMemo<DashboardMetricsValue>(
    () => ({
      history,
      range,
      setRange,
      hasLivestream: !!storeState.livestream,
    }),
    [history, range, storeState.livestream],
  );

  return (
    <DashboardMetricsContext.Provider value={value}>
      {children}
    </DashboardMetricsContext.Provider>
  );
}
