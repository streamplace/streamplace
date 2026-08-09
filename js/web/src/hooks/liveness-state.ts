const STALE_AFTER_SECONDS = 10;
const OFFLINE_AFTER_SECONDS = 300;

export type Liveness = "loading" | "live" | "stale" | "offline" | "never-live";

export function latestActivityAgeSeconds(
  {
    segmentStartTime,
    segmentDurationNanoseconds,
    lastSeenAt,
  }: {
    segmentStartTime?: string | null;
    segmentDurationNanoseconds?: number | null;
    lastSeenAt?: string | null;
  },
  nowMs = Date.now(),
): number {
  let latestActivityMs = Number.NEGATIVE_INFINITY;
  const segmentStartMs = segmentStartTime
    ? Date.parse(segmentStartTime)
    : Number.NaN;
  if (Number.isFinite(segmentStartMs)) {
    const durationMs = Math.max(0, segmentDurationNanoseconds ?? 0) / 1e6;
    latestActivityMs = segmentStartMs + durationMs;
  }

  const lastSeenMs = lastSeenAt ? Date.parse(lastSeenAt) : Number.NaN;
  if (Number.isFinite(lastSeenMs)) {
    latestActivityMs = Math.max(latestActivityMs, lastSeenMs);
  }
  if (!Number.isFinite(latestActivityMs)) return 0;

  return Math.max(0, Math.floor((nowMs - latestActivityMs) / 1000));
}

export function deriveLiveness({
  endedAt,
  hasInitialResponse,
  hasReceivedSegment,
  hasLivestream,
  secondsSinceActivity,
}: {
  endedAt?: string | null;
  hasInitialResponse: boolean;
  hasReceivedSegment: boolean;
  hasLivestream: boolean;
  secondsSinceActivity: number;
}): Liveness {
  if (endedAt) return "offline";
  if (!hasInitialResponse && !hasReceivedSegment && !hasLivestream) {
    return "loading";
  }
  if (!hasReceivedSegment && !hasLivestream) return "never-live";
  if (secondsSinceActivity >= OFFLINE_AFTER_SECONDS) return "offline";
  if (secondsSinceActivity >= STALE_AFTER_SECONDS) return "stale";
  return "live";
}
