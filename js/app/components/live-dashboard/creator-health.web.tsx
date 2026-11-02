import { useLivestreamStore, useTheme, zero } from "@streamplace/components";
import { breakpoints } from "@streamplace/components/src/lib/theme/tokens";
import { getSegmentMetrics } from "@streamplace/core";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useWindowDimensions } from "react-native";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  CREATOR_METRIC_RANGES,
  type CreatorMetricPoint,
  type CreatorMetricRange,
} from "./creator-metrics";
import { useCreatorMetrics } from "./creator-metrics-provider.web";

type Metric = Exclude<keyof CreatorMetricPoint, "at">;
type MetricGroup = "video" | "audience";

const metrics: {
  id: Metric;
  label: string;
  unit: string;
  group: MetricGroup;
  format: (value: number) => string;
}[] = [
  {
    id: "bitrateKbps",
    label: "Bitrate",
    unit: "kbps",
    group: "video",
    format: (value) => `${(value / 1000).toFixed(2)} Mbps`,
  },
  {
    id: "segmentMs",
    label: "Segment Δ",
    unit: "ms",
    group: "video",
    format: (value) => `${Math.round(value)} ms`,
  },
  {
    id: "fps",
    label: "FPS",
    unit: "fps",
    group: "video",
    format: (value) => `${value.toFixed(1)} fps`,
  },
  {
    id: "viewers",
    label: "Viewers",
    unit: "viewers",
    group: "audience",
    format: (value) => String(Math.round(value)),
  },
  {
    id: "chatRate",
    label: "Chat rate",
    unit: "msg/min",
    group: "audience",
    format: (value) => `${Math.round(value)} msg/min`,
  },
];

function compact(value: number): string {
  if (Math.abs(value) >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (Math.abs(value) >= 1000) return `${(value / 1000).toFixed(1)}k`;
  return String(Math.round(value));
}

export default function CreatorHealth({
  onShowProblems,
}: {
  onShowProblems: () => void;
}) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const { width } = useWindowDimensions();
  const { history, range, setRange, status } = useCreatorMetrics();
  const problems = useLivestreamStore((state) => state.problems);
  const segment = useLivestreamStore((state) => state.segment);
  const [active, setActive] = useState<Set<Metric>>(
    () => new Set(["bitrateKbps"]),
  );
  const last = history[history.length - 1];
  const resolution = status.readingsStale
    ? null
    : getSegmentMetrics(segment).resolution;
  const chartEnd = last?.at ?? Date.now();
  const borderWidth = zero.borders.width.thin.borderWidth;
  const lineWidth = zero.borders.width.medium.borderWidth;
  const colors: Record<Metric, string> = {
    bitrateKbps: theme.colors.success,
    segmentMs: theme.colors.warning,
    fps: theme.colors.secondary,
    viewers: theme.colors.primary,
    chatRate: theme.colors.info,
  };
  const statusLabel =
    status.incoming === null
      ? status.phase === "reconnecting"
        ? t("creator-readings-stale")
        : t("creator-readings-unavailable", {
            defaultValue: "Readings unavailable",
          })
      : status.incoming === false
        ? t("creator-status-waiting")
        : status.connectionQuality === "pre-live"
          ? t("connection-pre-live", { defaultValue: "Previewing" })
          : !status.hasTimingReadings
            ? t("connection-measuring", { defaultValue: "Measuring" })
            : status.connectionQuality === "good"
              ? t("connection-excellent", { defaultValue: "Excellent" })
              : status.connectionQuality === "degraded"
                ? t("connection-good", { defaultValue: "Good" })
                : t("connection-poor", { defaultValue: "Poor" });
  const statusColor =
    status.incoming !== true || !status.hasTimingReadings
      ? theme.colors.text3
      : status.connectionQuality === "good"
        ? theme.colors.success
        : status.connectionQuality === "poor"
          ? theme.colors.danger
          : theme.colors.warning;
  const activeGroups = (["video", "audience"] as const).filter((group) =>
    metrics.some((metric) => metric.group === group && active.has(metric.id)),
  );
  const stats = useMemo(() => {
    const result: Partial<Record<Metric, { average: number; peak: number }>> =
      {};
    for (const metric of metrics) {
      if (!active.has(metric.id)) continue;
      let total = 0;
      let count = 0;
      let peak = 0;
      for (const point of history) {
        const value = point[metric.id];
        if (value === null) continue;
        total += value;
        count++;
        peak = Math.max(peak, value);
      }
      if (count) result[metric.id] = { average: total / count, peak };
    }
    return result;
  }, [history, active]);

  return (
    <div
      data-testid="creator-health"
      style={{
        display: "flex",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
        minWidth: 0,
        overflow: "auto",
        gap: theme.spacing[3],
        padding: theme.spacing[4],
        background: theme.colors.surface1,
        border: `${borderWidth}px solid ${theme.colors.borderSubtle}`,
        borderRadius: theme.borderRadius.lg,
        color: theme.colors.text1,
        ...theme.typography.universal.xs,
        lineHeight: `${theme.typography.universal.xs.lineHeight}px`,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          flexWrap: "wrap",
          gap: theme.spacing[2],
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            flexWrap: "wrap",
            gap: theme.spacing[2],
          }}
        >
          <strong>
            {t("stream-health", { defaultValue: "Stream Health" })}
          </strong>
          <span style={{ color: statusColor }}>{statusLabel}</span>
          <span style={{ color: theme.colors.text2 }}>
            Resolution {resolution ?? "—"}
          </span>
          {problems.length > 0 && (
            <button
              type="button"
              onClick={onShowProblems}
              style={{
                color: theme.colors.warning,
                background: "transparent",
                border: 0,
                minHeight: theme.touchTargets.minimum,
                cursor: "pointer",
                font: "inherit",
              }}
            >
              {problems.length} {problems.length === 1 ? "issue" : "issues"}
            </button>
          )}
        </div>
        <div
          role="group"
          aria-label="Health history range"
          style={{ display: "flex", gap: theme.spacing[1] }}
        >
          {(Object.keys(CREATOR_METRIC_RANGES) as CreatorMetricRange[]).map(
            (item) => (
              <button
                key={item}
                type="button"
                aria-pressed={range === item}
                onClick={() => setRange(item)}
                style={{
                  color:
                    range === item ? theme.colors.text1 : theme.colors.text3,
                  background:
                    range === item ? theme.colors.surface3 : "transparent",
                  border: 0,
                  minWidth: theme.touchTargets.minimum,
                  minHeight: theme.touchTargets.minimum,
                  borderRadius: theme.borderRadius.sm,
                  padding: theme.spacing[1],
                  cursor: "pointer",
                  font: "inherit",
                }}
              >
                {item}
              </button>
            ),
          )}
        </div>
      </div>

      <div
        style={{
          display: "flex",
          flex: "1 0 auto",
          minHeight: theme.spacing[16] * 2,
          minWidth: 0,
          gap: theme.spacing[2],
          flexDirection:
            width < breakpoints.md ||
            (activeGroups.length > 1 && active.size > 2)
              ? "column"
              : "row",
        }}
      >
        {activeGroups.map((group) => {
          const selected = metrics.filter(
            (metric) => metric.group === group && active.has(metric.id),
          );
          const measured = selected.filter((metric) =>
            history.some((point) => point[metric.id] !== null),
          );
          return (
            <div
              key={group}
              style={{
                flex: 1,
                minWidth: 0,
                minHeight: theme.spacing[16] * 2,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
              aria-label={
                group === "video" ? "Video metrics" : "Audience metrics"
              }
            >
              {measured.length === 0 ? (
                <span style={{ color: theme.colors.text3 }}>
                  Waiting for readings
                </span>
              ) : (
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart
                    data={history}
                    margin={{
                      top: theme.spacing[1],
                      right: theme.spacing[1],
                      bottom: 0,
                      left: 0,
                    }}
                  >
                    <CartesianGrid
                      stroke={theme.colors.borderSubtle}
                      strokeDasharray={`${theme.spacing[1]} ${theme.spacing[1]}`}
                    />
                    <XAxis
                      dataKey="at"
                      type="number"
                      domain={[
                        chartEnd - CREATOR_METRIC_RANGES[range],
                        chartEnd,
                      ]}
                      tickFormatter={(value) =>
                        new Date(value).toLocaleTimeString([], {
                          minute: "2-digit",
                          second: "2-digit",
                        })
                      }
                      tick={{
                        fontSize: theme.typography.universal.xs.fontSize,
                        fill: theme.colors.text3,
                      }}
                      minTickGap={theme.spacing[8]}
                    />
                    {measured.map((metric, index) => (
                      <YAxis
                        key={metric.id}
                        yAxisId={metric.id}
                        orientation={index % 2 === 0 ? "left" : "right"}
                        width={theme.spacing[12]}
                        tickFormatter={compact}
                        tick={{
                          fontSize: theme.typography.universal.xs.fontSize,
                          fill: theme.colors.text3,
                        }}
                        label={{
                          value: metric.unit,
                          angle: index % 2 === 0 ? -90 : 90,
                          position:
                            index % 2 === 0 ? "insideLeft" : "insideRight",
                          fill: colors[metric.id],
                          fontSize: theme.typography.universal.xs.fontSize,
                        }}
                      />
                    ))}
                    <Tooltip
                      contentStyle={{
                        background: theme.colors.surface2,
                        border: `${borderWidth}px solid ${theme.colors.border}`,
                        borderRadius: theme.borderRadius.md,
                        color: theme.colors.text1,
                      }}
                      labelFormatter={(value) =>
                        new Date(Number(value)).toLocaleTimeString()
                      }
                      formatter={(value, name) => {
                        const metric = metrics.find((item) => item.id === name);
                        return [
                          metric && typeof value === "number"
                            ? metric.format(value)
                            : "—",
                          metric?.label ?? String(name),
                        ];
                      }}
                    />
                    {measured.map((metric) => (
                      <Line
                        key={metric.id}
                        yAxisId={metric.id}
                        type="monotone"
                        dataKey={metric.id}
                        name={metric.id}
                        stroke={colors[metric.id]}
                        strokeWidth={lineWidth}
                        dot={false}
                        connectNulls={false}
                        isAnimationActive={false}
                      />
                    ))}
                  </LineChart>
                </ResponsiveContainer>
              )}
            </div>
          );
        })}
        {activeGroups.length === 0 && (
          <span style={{ alignSelf: "center", color: theme.colors.text3 }}>
            Select a metric to display its chart.
          </span>
        )}
      </div>

      <div style={{ display: "flex", flexWrap: "wrap", gap: theme.spacing[2] }}>
        {metrics.map((metric) => {
          const selected = active.has(metric.id);
          const value = last?.[metric.id] ?? null;
          return (
            <button
              key={metric.id}
              type="button"
              aria-pressed={selected}
              onClick={() =>
                setActive((previous) => {
                  const next = new Set(previous);
                  if (next.has(metric.id)) next.delete(metric.id);
                  else next.add(metric.id);
                  return next;
                })
              }
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: theme.spacing[1],
                color: selected ? colors[metric.id] : theme.colors.text2,
                background: selected ? theme.colors.surface3 : "transparent",
                border: `${borderWidth}px solid ${selected ? colors[metric.id] : theme.colors.border}`,
                borderRadius: theme.borderRadius.md,
                minHeight: theme.touchTargets.minimum,
                padding: `${theme.spacing[1]}px ${theme.spacing[2]}px`,
                cursor: "pointer",
                font: "inherit",
                fontVariantNumeric: "tabular-nums",
              }}
            >
              <span>{metric.label}</span>
              <strong>{value === null ? "—" : metric.format(value)}</strong>
            </button>
          );
        })}
      </div>
      {metrics.some((metric) => active.has(metric.id) && stats[metric.id]) && (
        <div
          style={{
            display: "flex",
            flexWrap: "wrap",
            gap: theme.spacing[3],
            color: theme.colors.text2,
            fontVariantNumeric: "tabular-nums",
          }}
        >
          {metrics.map((metric) => {
            const summary = stats[metric.id];
            return (
              summary && (
                <span key={metric.id}>
                  {metric.label} avg {metric.format(summary.average)} · peak{" "}
                  {metric.format(summary.peak)}
                </span>
              )
            );
          })}
        </div>
      )}
    </div>
  );
}
