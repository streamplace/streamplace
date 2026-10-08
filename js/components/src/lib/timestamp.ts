/**
 * Temporal references, YouTube-style: `#t=5m5s` in the URL fragment loads
 * playback starting at that point in the stream.
 *
 * Parsing and formatting are pure helpers. The two `*StartTimeTarget`
 * functions convert a requested offset into a position on the media timeline,
 * given whatever seek window the player currently has:
 *
 * - With a stream-start anchor (the livestream record's `createdAt`) the
 *   offset names a fixed point in time: `anchor + offset`. This is what makes
 *   shared `#t=` links stable — everyone computing the same wall-clock point.
 * - Without an anchor (direct media URLs, VOD playlists) the offset is simply
 *   the position in the media itself.
 *
 * Live windows are short (the server retains only a few segments), so points
 * older than the available window are clamped to its start, and points at or
 * past the live edge return null (start live, the default behavior).
 */

const TIMESTAMP_RE = /^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s?)?$/;

/**
 * Parse a YouTube-style timestamp: `305`, `305s`, `5m5s`, `1h2m3s`, `1h`,
 * `90m`. Units are optional but at least one must be present; a bare number
 * means seconds. Returns null for anything unparseable.
 */
export function parseTimestamp(spec: string | null | undefined): number | null {
  if (!spec) {
    return null;
  }
  const match = spec.trim().toLowerCase().match(TIMESTAMP_RE);
  if (!match || (!match[1] && !match[2] && !match[3])) {
    return null;
  }
  const hours = match[1] ? parseInt(match[1], 10) : 0;
  const minutes = match[2] ? parseInt(match[2], 10) : 0;
  const seconds = match[3] ? parseFloat(match[3]) : 0;
  return hours * 3600 + minutes * 60 + seconds;
}

/**
 * Format a duration in seconds as a canonical `#t=` timestamp: `90s`,
 * `5m5s`, `1h2m3s`. Zero-valued units are omitted.
 */
export function formatTimestamp(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  let out = "";
  if (hours > 0) {
    out += `${hours}h`;
  }
  if (minutes > 0) {
    out += `${minutes}m`;
  }
  out += `${secs}s`;
  return out;
}

/**
 * Extract a temporal reference from a URL fragment: `#t=5m5s` (also tolerates
 * `#other=1&t=5m5s`). Returns the offset in seconds, or null.
 */
export function parseTimeHash(hash: string): number | null {
  if (!hash) {
    return null;
  }
  const match = hash.match(/[#&]t=([^&]+)/);
  if (!match) {
    return null;
  }
  return parseTimestamp(decodeURIComponent(match[1]));
}

/** Minimal shape of an hls.js fragment, for wall-clock mapping. */
export type TimestampFragment = {
  /** Position on the media timeline, in seconds. */
  start: number;
  duration: number;
  /** Wall-clock time of the fragment start, in epoch milliseconds. */
  programDateTime?: number;
};

/**
 * Compute the media-timeline position to seek an hls.js stream to, honoring a
 * temporal reference. Returns null when no seek should happen (reference at or
 * past the live edge, or nothing to seek with).
 *
 * `fragments` are the current level's fragments; hls.js `Fragment` objects are
 * structurally compatible.
 */
export function hlsStartTimeTarget(opts: {
  startTime: number;
  streamStartMs?: number | null;
  fragments: TimestampFragment[];
  live: boolean;
  duration?: number;
}): number | null {
  const { startTime, streamStartMs, live } = opts;
  const frags = opts.fragments.filter((f) => f && isFinite(f.start));
  if (frags.length === 0) {
    return null;
  }
  const first = frags[0];
  const last = frags[frags.length - 1];
  const windowStart = first.start;
  const windowEnd = last.start + last.duration;

  // VOD playlist: the offset is just the position in the media.
  if (!live) {
    const total =
      opts.duration && opts.duration > 0 ? opts.duration : windowEnd;
    return Math.min(Math.max(startTime, 0), total);
  }

  const anchored =
    streamStartMs != null &&
    frags.some((f) => typeof f.programDateTime === "number");

  if (anchored) {
    // Fixed point in time: map wall clock onto the media timeline through the
    // fragments' program dates (exact; no client clock involved).
    const a0 = frags.find(
      (f) => typeof f.programDateTime === "number",
    ) as TimestampFragment;
    const a0Wall = a0.programDateTime!;
    const targetWall = streamStartMs! + startTime * 1000;
    if (targetWall <= a0Wall) {
      // Older than anything retained: clamp to the earliest available point.
      return windowStart;
    }
    const lastWall =
      (typeof last.programDateTime === "number"
        ? last.programDateTime
        : a0Wall + (last.start - a0.start) * 1000) +
      last.duration * 1000;
    if (targetWall >= lastWall) {
      // At or past the live edge: keep default live behavior.
      return null;
    }
    return a0.start + (targetWall - a0Wall) / 1000;
  }

  // Live window without a stream-start anchor: measure the offset from the
  // start of the available window.
  return Math.min(Math.max(windowStart + startTime, windowStart), windowEnd);
}

/**
 * Compute the position to seek a plain video element to (progressive
 * mp4/webm, or Safari's native HLS), honoring a temporal reference.
 *
 * The element exposes no program dates, so live wall-clock mapping
 * approximates the live edge as "now" — client/server clock skew shifts the
 * landing point accordingly. Returns null when no seek should happen.
 */
export function elementStartTimeTarget(opts: {
  startTime: number;
  streamStartMs?: number | null;
  seekable: {
    length: number;
    start(i: number): number;
    end(i: number): number;
  };
  duration: number;
}): number | null {
  const { startTime, streamStartMs } = opts;
  const total =
    isFinite(opts.duration) && opts.duration > 0 ? opts.duration : null;

  if (total != null && streamStartMs == null) {
    // Plain VOD file: the offset is just the position in the file.
    return Math.min(Math.max(startTime, 0), total);
  }

  if (opts.seekable.length === 0) {
    return total != null ? Math.min(Math.max(startTime, 0), total) : null;
  }

  const start = opts.seekable.start(0);
  const end = opts.seekable.end(opts.seekable.length - 1);

  if (streamStartMs != null) {
    const target =
      end - (Date.now() - (streamStartMs + startTime * 1000)) / 1000;
    if (target >= end - 0.5) {
      // At or past the live edge: keep default live behavior.
      return null;
    }
    return Math.min(Math.max(target, start), end);
  }

  return Math.min(Math.max(start + startTime, start), end);
}
