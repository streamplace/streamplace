import { getStoreFromContext } from "@streamplace/components";
import { useLiveDashboardStatus } from "hooks/useLiveDashboardStatus";
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  appendCreatorMetric,
  CREATOR_METRIC_INTERVAL_MS,
  creatorMetricsInRange,
  sampleCreatorMetrics,
  type CreatorMetricPoint,
  type CreatorMetricRange,
} from "./creator-metrics";

type CreatorMetrics = {
  history: CreatorMetricPoint[];
  range: CreatorMetricRange;
  setRange: (range: CreatorMetricRange) => void;
  status: ReturnType<typeof useLiveDashboardStatus>;
};

const CreatorMetricsContext = createContext<CreatorMetrics | null>(null);

export default function CreatorMetricsProvider({
  children,
}: {
  children: ReactNode;
}) {
  const store = getStoreFromContext();
  const status = useLiveDashboardStatus();
  const statusRef = useRef(status);
  statusRef.current = status;
  const [history, setHistory] = useState<CreatorMetricPoint[]>([]);
  const [range, setRange] = useState<CreatorMetricRange>("2m");

  useEffect(() => {
    const timer = setInterval(() => {
      const point = sampleCreatorMetrics(
        store.getState(),
        Date.now(),
        statusRef.current.hasTimingReadings
          ? statusRef.current.timeBetweenSegments
          : null,
      );
      setHistory((previous) => appendCreatorMetric(previous, point));
    }, CREATOR_METRIC_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [store]);

  const visibleHistory = useMemo(
    () =>
      creatorMetricsInRange(
        history,
        history[history.length - 1]?.at ?? Date.now(),
        range,
      ),
    [history, range],
  );
  const value = useMemo(
    () => ({ history: visibleHistory, range, setRange, status }),
    [visibleHistory, range, status],
  );
  return (
    <CreatorMetricsContext.Provider value={value}>
      {children}
    </CreatorMetricsContext.Provider>
  );
}

export function useCreatorMetrics() {
  const value = useContext(CreatorMetricsContext);
  if (!value)
    throw new Error("useCreatorMetrics requires CreatorMetricsProvider");
  return value;
}
