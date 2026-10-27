import type { place } from "streamplace";
import type { LivestreamState } from "./state";

export function isSegmentFresh(
  startTime: string | undefined,
  now: number,
): boolean {
  if (!startTime) return false;
  const age = now - Date.parse(startTime);
  return Number.isFinite(age) && age >= 0 && age < 10_000;
}

export function deriveLivestreamStatus(
  state: Pick<
    LivestreamState,
    | "websocketConnected"
    | "segment"
    | "livestream"
    | "profile"
    | "hasReceivedSegment"
  >,
  now: number,
) {
  const connected = state.websocketConnected;
  const fresh = isSegmentFresh(state.segment?.startTime, now);
  const hasSnapshot = !!(
    state.profile ||
    state.livestream ||
    state.hasReceivedSegment
  );
  const phase = !connected
    ? hasSnapshot
      ? "reconnecting"
      : "connecting"
    : state.livestream?.record.endedAt !== undefined
      ? "ended"
      : state.livestream
        ? "live"
        : fresh
          ? "ready"
          : "waiting";

  return {
    phase,
    incoming: connected ? fresh : null,
    canAnnounce: connected && fresh,
    readingsStale: !connected || !fresh,
  } as const;
}

function positive(value: number | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value) && value > 0;
}

export function getSegmentMetrics(segment: place.stream.segment.Main | null) {
  const video = segment?.video?.[0];
  const framerate = video?.framerate;
  const bitrate =
    segment && positive(segment.size) && positive(segment.duration)
      ? (segment.size * 8) / (segment.duration / 1_000_000_000) / 1000
      : null;
  const fps =
    framerate && positive(framerate.num) && positive(framerate.den)
      ? framerate.num / framerate.den
      : null;

  return {
    bitrateKbps: positive(bitrate ?? undefined) ? bitrate : null,
    resolution:
      positive(video?.width) &&
      positive(video?.height) &&
      Number.isInteger(video.width) &&
      Number.isInteger(video.height)
        ? `${video.width}x${video.height}`
        : null,
    fps: positive(fps ?? undefined) ? fps : null,
  };
}
