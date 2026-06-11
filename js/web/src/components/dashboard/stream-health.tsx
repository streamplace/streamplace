import { cn } from "@/lib/utils";
import type { LivestreamStore } from "@streamplace/core";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DashboardMetricsPoint } from "./dashboard-metrics";
import { DASHBOARD_RANGES, useDashboardMetrics } from "./dashboard-metrics";

type MetricGroup = "video" | "audience";
type AxisId = "primary" | "secondary";
type MetricId = "bitrate" | "viewers" | "segmentTiming" | "chatRate" | "fps";

interface MetricConfig {
  id: MetricId;
  label: string;
  group: MetricGroup;
  /** Which y-axis this metric renders against. The audience chart
   *  ignores this and uses a single axis. */
  yAxisId: AxisId;
  color: string;
  format: (v: number) => string;
}

const METRICS: Record<MetricId, MetricConfig> = {
  bitrate: {
    id: "bitrate",
    label: "Bitrate",
    group: "video",
    yAxisId: "primary",
    color: "#22c55e",
    format: (v) => `${(v / 1000).toFixed(2)} Mbps`,
  },
  segmentTiming: {
    id: "segmentTiming",
    label: "Segment Δ",
    group: "video",
    yAxisId: "primary",
    color: "#f59e0b",
    format: (v) => `${Math.round(v)} ms`,
  },
  fps: {
    id: "fps",
    label: "FPS",
    group: "video",
    yAxisId: "secondary",
    color: "#ec4899",
    format: (v) => `${Math.round(v)} fps`,
  },
  viewers: {
    id: "viewers",
    label: "Viewers",
    group: "audience",
    yAxisId: "primary",
    color: "#a78bfa",
    format: (v) => `${Math.round(v)}`,
  },
  chatRate: {
    id: "chatRate",
    label: "Chat rate",
    group: "audience",
    yAxisId: "primary",
    color: "#06b6d4",
    format: (v) => `${v.toFixed(1)} msg/min`,
  },
};

const METRIC_KEYS = Object.keys(METRICS) as MetricId[];

/**
 * Compact number formatter for chart axis labels: 1500 → "1.5k",
 * 2400 → "2.4k", 6000 → "6k", 12000 → "12k", 2_500_000 → "2.5M". Anything
 * under 1000 is shown as-is. The unit is implicit — viewers read the chips
 * to know whether the axis is kbps, ms, msg/min, etc.
 */
function compactNumber(v: number): string {
  const abs = Math.abs(v);
  if (abs >= 1_000_000) {
    return `${(v / 1_000_000).toFixed(abs >= 10_000_000 ? 0 : 1)}M`;
  }
  if (abs >= 1_000) {
    return `${(v / 1_000).toFixed(abs >= 10_000 ? 0 : 1)}k`;
  }
  return v.toString();
}

type ConnectionQuality = "good" | "degraded" | "poor" | "pre-live";

function getConnectionQuality(
  segmentTiming: number,
  hasLivestream: boolean,
): ConnectionQuality {
  if (!hasLivestream) return "pre-live";
  if (segmentTiming === 0) return "poor";
  if (segmentTiming <= 1500) return "good";
  if (segmentTiming <= 3000) return "degraded";
  return "poor";
}

/**
 * Stream Health widget. All heavy lifting (metric sampling, history
 * buffer, range management) lives in `DashboardMetricsProvider` higher
 * in the tree — this component only reads the pre-computed data and
 * renders the charts, chips, and footer.
 */
export function StreamHealthWidget({ store }: { store: LivestreamStore }) {
  const { t } = useTranslation("common");
  const { history, range, setRange, hasLivestream } = useDashboardMetrics();

  const [activeMetrics, setActiveMetrics] = useState<Set<MetricId>>(
    () => new Set<MetricId>(["bitrate"]),
  );

  // Track the chart container's aspect ratio. The two charts split along
  // the longest axis — landscape → vertical split (left/right),
  // portrait → horizontal split (top/bottom).
  const chartContainerRef = useRef<HTMLDivElement>(null);
  const [isLandscape, setIsLandscape] = useState(true);
  useEffect(() => {
    if (!chartContainerRef.current) return;
    const update = () => {
      if (!chartContainerRef.current) return;
      const { width, height } =
        chartContainerRef.current.getBoundingClientRect();
      setIsLandscape(width >= height);
    };
    update();
    const ro = new ResizeObserver(update);
    ro.observe(chartContainerRef.current);
    return () => ro.disconnect();
  }, []);

  const lastPoint = history[history.length - 1] ?? emptyPoint(Date.now());
  const connectionQuality = useMemo(
    () => getConnectionQuality(lastPoint.segmentTiming, hasLivestream),
    [hasLivestream, lastPoint.segmentTiming],
  );

  const qualityStyles = {
    good: { dot: "bg-green-400", text: "text-green-400" },
    degraded: { dot: "bg-amber-400", text: "text-amber-400" },
    poor: { dot: "bg-red-400", text: "text-red-400" },
    "pre-live": { dot: "bg-blue-400", text: "text-blue-400" },
  }[connectionQuality];
  const qualityLabel = {
    good: t("connection-excellent", { defaultValue: "Excellent" }),
    degraded: t("connection-good", { defaultValue: "Good" }),
    poor: t("connection-poor", { defaultValue: "Poor" }),
    "pre-live": t("connection-pre-live", { defaultValue: "Not live" }),
  }[connectionQuality];

  const toggleMetric = (id: MetricId) => {
    setActiveMetrics((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const stats = useMemo(() => {
    const out: Record<string, { current: number; avg: number; peak: number }> =
      {};
    for (const id of activeMetrics) {
      const values = history.map((d) => d[id]);
      const current = values[values.length - 1] ?? 0;
      const avg =
        values.length > 0
          ? values.reduce((a, b) => a + b, 0) / values.length
          : 0;
      const peak = values.length > 0 ? Math.max(...values) : 0;
      out[id] = { current, avg, peak };
    }
    return out;
  }, [history, activeMetrics]);

  const activeVideoIds = METRIC_KEYS.filter(
    (id) => activeMetrics.has(id) && METRICS[id].group === "video",
  );
  const activeAudienceIds = METRIC_KEYS.filter(
    (id) => activeMetrics.has(id) && METRICS[id].group === "audience",
  );

  const showVideo = activeVideoIds.length > 0;
  const showAudience = activeAudienceIds.length > 0;

  return (
    <div className="h-full rounded-b-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 flex flex-col gap-3 min-h-0">
      {/* Header */}
      <div className="flex items-center justify-between gap-2 shrink-0">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold">
            {t("stream-health", { defaultValue: "Stream Health" })}
          </h3>
          <div className={`w-2 h-2 rounded-full ${qualityStyles.dot}`} />
          <span className={`text-xs font-medium ${qualityStyles.text}`}>
            {qualityLabel}
          </span>
        </div>
        <div className="flex items-center gap-0.5">
          {DASHBOARD_RANGES.map((r) => (
            <button
              key={r.id}
              type="button"
              onClick={() => setRange(r.id)}
              className={cn(
                "px-1.5 py-0.5 text-[10px] rounded transition-colors",
                range === r.id
                  ? "bg-[var(--color-accent)]/20 text-[var(--color-accent)] font-semibold"
                  : "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]",
              )}
              aria-pressed={range === r.id}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {/* Chart container. When both video and audience metrics are
          active, splits in half along the longest axis. When only one
          set is active, that chart fills the available space. */}
      <div
        ref={chartContainerRef}
        className={cn(
          "flex-1 min-h-0 min-w-0 flex gap-1",
          isLandscape ? "flex-row" : "flex-col",
        )}
      >
        {showVideo && (
          <VideoChart history={history} activeIds={activeVideoIds} />
        )}
        {showAudience && (
          <AudienceChart history={history} activeIds={activeAudienceIds} />
        )}
        {!showVideo && !showAudience && (
          <div className="flex-1 flex items-center justify-center text-xs text-[var(--color-fg-muted)] border border-dashed border-[var(--color-border)] rounded">
            {t("select-metric", { defaultValue: "Click a metric to overlay" })}
          </div>
        )}
      </div>

      {/* Metric chips — each shows its current value always, and toggles
          that metric in/out of the chart when clicked. */}
      <div className="flex flex-wrap gap-1.5 shrink-0">
        {METRIC_KEYS.map((id) => {
          const metric = METRICS[id];
          const isActive = activeMetrics.has(id);
          const current = lastPoint[id];
          return (
            <button
              key={id}
              type="button"
              onClick={() => toggleMetric(id)}
              className={cn(
                "flex items-center gap-1.5 px-2 py-1 rounded-md border text-xs transition-colors",
                isActive
                  ? "border-transparent"
                  : "border-[var(--color-border)] text-[var(--color-fg-muted)] hover:border-[var(--color-border-strong)] hover:text-[var(--color-fg)]",
              )}
              style={
                isActive
                  ? {
                      backgroundColor: `${metric.color}22`,
                      color: metric.color,
                      borderColor: `${metric.color}55`,
                    }
                  : undefined
              }
              aria-pressed={isActive}
              title={
                isActive
                  ? t("metric-overlay-dismiss", {
                      defaultValue: "Click to dismiss from chart",
                    })
                  : t("metric-overlay-show", {
                      defaultValue: "Click to overlay on chart",
                    })
              }
            >
              <div
                className="w-1.5 h-1.5 rounded-full shrink-0"
                style={{
                  backgroundColor: metric.color,
                  opacity: isActive ? 1 : 0.45,
                }}
              />
              <span className="font-medium">{metric.label}</span>
              <span
                className={cn(
                  "tabular-nums",
                  isActive ? "font-semibold" : "text-[var(--color-fg)]",
                )}
              >
                {metric.format(current)}
              </span>
            </button>
          );
        })}
      </div>

      {/* Footer stats — avg + peak per active metric (current is in the chip) */}
      {activeMetrics.size > 0 && (
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs shrink-0">
          {METRIC_KEYS.filter((id) => activeMetrics.has(id)).map((id) => {
            const metric = METRICS[id];
            const s = stats[id];
            if (!s) return null;
            return (
              <div key={id} className="flex items-center gap-1.5 tabular-nums">
                <div
                  className="w-1.5 h-1.5 rounded-full"
                  style={{ backgroundColor: metric.color }}
                />
                <span className="text-[var(--color-fg-muted)]">
                  {metric.label}
                </span>
                <span className="text-[10px] text-[var(--color-fg-muted)]">
                  avg {metric.format(s.avg)}
                </span>
                <span className="text-[10px] text-[var(--color-fg-muted)]">
                  peak {metric.format(s.peak)}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

interface ChartProps {
  history: DashboardMetricsPoint[];
  activeIds: MetricId[];
}

function VideoChart({ history, activeIds }: ChartProps) {
  const { t } = useTranslation("common");
  const hasSecondary = activeIds.some(
    (id) => METRICS[id].yAxisId === "secondary",
  );
  return (
    <div className="flex-1 min-w-0 min-h-0 flex flex-col">
      <div className="text-[10px] text-[var(--color-fg-muted)] uppercase tracking-wider px-1 mb-0.5 shrink-0">
        {t("video", { defaultValue: "Video" })}
      </div>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={history}
          margin={{ top: 4, right: 6, bottom: 4, left: 6 }}
        >
          <CartesianGrid
            stroke="rgba(255,255,255,0.06)"
            strokeDasharray="3 3"
          />
          <XAxis
            dataKey="t"
            tickFormatter={(t) =>
              new Date(t).toLocaleTimeString([], {
                minute: "2-digit",
                second: "2-digit",
              })
            }
            tick={{ fontSize: 9, fill: "var(--color-fg-muted)" }}
            stroke="var(--color-border)"
            minTickGap={32}
          />
          <YAxis
            yAxisId="primary"
            orientation="left"
            tick={{ fontSize: 9, fill: "var(--color-fg-muted)" }}
            stroke="var(--color-border)"
            width={36}
            tickFormatter={compactNumber}
          />
          {hasSecondary && (
            <YAxis
              yAxisId="secondary"
              orientation="right"
              tick={{ fontSize: 9, fill: "var(--color-fg-muted)" }}
              stroke="var(--color-border)"
              width={32}
              tickFormatter={compactNumber}
            />
          )}
          <Tooltip
            contentStyle={{
              backgroundColor: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: 6,
              fontSize: 12,
            }}
            labelFormatter={(t) => new Date(t as number).toLocaleTimeString()}
            formatter={(value, name) => {
              const id = name as MetricId;
              const metric = METRICS[id];
              if (!metric) return [value as string, name as string];
              return [metric.format(value as number), metric.label];
            }}
          />
          {activeIds.map((id) => {
            const metric = METRICS[id];
            return (
              <Line
                key={id}
                yAxisId={metric.yAxisId}
                type="monotone"
                dataKey={id}
                name={id}
                stroke={metric.color}
                strokeWidth={1.5}
                dot={false}
                isAnimationActive={false}
              />
            );
          })}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

function AudienceChart({ history, activeIds }: ChartProps) {
  const { t } = useTranslation("common");
  return (
    <div className="flex-1 min-w-0 min-h-0 flex flex-col">
      <div className="text-[10px] text-[var(--color-fg-muted)] uppercase tracking-wider px-1 mb-0.5 shrink-0">
        {t("audience", { defaultValue: "Audience" })}
      </div>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={history}
          margin={{ top: 4, right: 6, bottom: 4, left: 6 }}
        >
          <CartesianGrid
            stroke="rgba(255,255,255,0.06)"
            strokeDasharray="3 3"
          />
          <XAxis
            dataKey="t"
            tickFormatter={(t) =>
              new Date(t).toLocaleTimeString([], {
                minute: "2-digit",
                second: "2-digit",
              })
            }
            tick={{ fontSize: 9, fill: "var(--color-fg-muted)" }}
            stroke="var(--color-border)"
            minTickGap={32}
          />
          <YAxis
            tick={{ fontSize: 9, fill: "var(--color-fg-muted)" }}
            stroke="var(--color-border)"
            width={32}
            tickFormatter={compactNumber}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: "var(--color-bg)",
              border: "1px solid var(--color-border)",
              borderRadius: 6,
              fontSize: 12,
            }}
            labelFormatter={(t) => new Date(t as number).toLocaleTimeString()}
            formatter={(value, name) => {
              const id = name as MetricId;
              const metric = METRICS[id];
              if (!metric) return [value as string, name as string];
              return [metric.format(value as number), metric.label];
            }}
          />
          {activeIds.map((id) => {
            const metric = METRICS[id];
            return (
              <Line
                key={id}
                type="monotone"
                dataKey={id}
                name={id}
                stroke={metric.color}
                strokeWidth={1.5}
                dot={false}
                isAnimationActive={false}
              />
            );
          })}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

function emptyPoint(t: number): DashboardMetricsPoint {
  return { t, bitrate: 0, viewers: 0, segmentTiming: 0, chatRate: 0, fps: 0 };
}
