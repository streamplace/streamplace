import {
  deriveLivestreamStatus,
  getSegmentMetrics,
  type LivestreamState,
} from "@streamplace/core";

export const CREATOR_METRIC_INTERVAL_MS = 2000;
const HISTORY_DURATION_MS = 25 * 60_000;
const HISTORY_LIMIT = HISTORY_DURATION_MS / CREATOR_METRIC_INTERVAL_MS;

export const CREATOR_METRIC_RANGES = {
  "1m": 60_000,
  "2m": 2 * 60_000,
  "5m": 5 * 60_000,
  "10m": 10 * 60_000,
  "25m": HISTORY_DURATION_MS,
} as const;
export type CreatorMetricRange = keyof typeof CREATOR_METRIC_RANGES;

export interface CreatorMetricPoint {
  at: number;
  bitrateKbps: number | null;
  fps: number | null;
  segmentMs: number | null;
  viewers: number | null;
  chatRate: number | null;
}

function emptyMetricPoint(at: number): CreatorMetricPoint {
  return {
    at,
    bitrateKbps: null,
    fps: null,
    segmentMs: null,
    viewers: null,
    chatRate: null,
  };
}

export function sampleCreatorMetrics(
  state: LivestreamState,
  now: number,
  segmentMs: number | null,
): CreatorMetricPoint {
  if (!state.websocketConnected) return emptyMetricPoint(now);
  const { readingsStale } = deriveLivestreamStatus(state, now);
  const media = readingsStale ? null : getSegmentMetrics(state.segment);
  const viewers = state.viewers;
  let chatRate = 0;
  for (const message of state.chat) {
    const at = Date.parse(message.indexedAt);
    if (
      message.author.did !== "did:sys:system" &&
      Number.isFinite(at) &&
      at > now - 60_000 &&
      at <= now
    )
      chatRate++;
  }
  return {
    at: now,
    bitrateKbps: media?.bitrateKbps ?? null,
    fps: media?.fps ?? null,
    segmentMs:
      !readingsStale &&
      segmentMs !== null &&
      Number.isFinite(segmentMs) &&
      segmentMs > 0
        ? segmentMs
        : null,
    viewers:
      viewers !== null && Number.isInteger(viewers) && viewers >= 0
        ? viewers
        : null,
    chatRate,
  };
}

export function appendCreatorMetric(
  history: CreatorMetricPoint[],
  point: CreatorMetricPoint,
): CreatorMetricPoint[] {
  const cutoff = point.at - HISTORY_DURATION_MS;
  let start = 0;
  while (start < history.length && history[start].at < cutoff) start++;
  const next = history.slice(start);
  const last = next[next.length - 1];
  if (last && point.at - last.at > CREATOR_METRIC_INTERVAL_MS * 1.5) {
    next.push(emptyMetricPoint(last.at + CREATOR_METRIC_INTERVAL_MS));
  }
  next.push(point);
  return next.length > HISTORY_LIMIT ? next.slice(-HISTORY_LIMIT) : next;
}

export function creatorMetricsInRange(
  history: CreatorMetricPoint[],
  now: number,
  range: CreatorMetricRange,
): CreatorMetricPoint[] {
  const cutoff = now - CREATOR_METRIC_RANGES[range];
  return history.filter((point) => point.at >= cutoff && point.at <= now);
}
