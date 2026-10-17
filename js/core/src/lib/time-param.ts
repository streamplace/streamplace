/**
 * Parse a YouTube-style `?t=` playback start time into seconds.
 *
 * VOD watch URLs (`/:user/video/:tid?t=90`) carry the position the viewer
 * wants playback to start at. The parameter is accepted in the two shapes
 * YouTube does:
 *
 *   - a bare number of seconds: `t=90`, `t=90.5`
 *   - a duration string: `t=1h2m3s`, `t=2m30s`, `t=90s`
 *
 * Anything else — a missing value, a malformed string, a negative number,
 * or a value that overflows to Infinity — returns `null`.
 * Callers still have to clamp the result to the media's duration.
 *
 * The parameter is typed `unknown` because URL search params and
 * react-navigation route params arrive untyped at runtime (a repeated
 * `?t=1&t=2` is an array, for instance).
 */
export function parseTimeParam(value: unknown): number | null {
  if (typeof value === "number") {
    return Number.isFinite(value) && value >= 0 ? value : null;
  }
  if (typeof value !== "string") {
    return null;
  }
  const raw = value.trim();
  if (raw === "") {
    return null;
  }
  if (/^\d+(\.\d+)?$/.test(raw)) {
    const seconds = Number(raw);
    return Number.isFinite(seconds) ? seconds : null;
  }
  const match = raw.match(/^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/i);
  if (!match) {
    return null;
  }
  const [, hours, minutes, seconds] = match;
  if (hours === undefined && minutes === undefined && seconds === undefined) {
    return null;
  }
  const total =
    Number(hours ?? 0) * 3600 +
    Number(minutes ?? 0) * 60 +
    Number(seconds ?? 0);
  return Number.isFinite(total) ? total : null;
}
