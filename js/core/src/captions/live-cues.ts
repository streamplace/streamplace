import { place } from "streamplace";
import type { CaptionTrackOption } from "./tracks";

/** A live cue as the player keeps it. */
export interface LiveCaption {
  id: string;
  trackId: string;
  text: string;
  final: boolean;
  /** Cue start on the segment wall clock, ms since the epoch. */
  startMs: number;
  endMs: number;
  /** Local time of the last revision, ms since the epoch. */
  updatedAt: number;
}

/** Minimum reading time for the streamer's standalone OBS display. */
export const LIVE_CAPTION_HOLD_MS = 5000;

export interface CaptionClock {
  startMs: number;
  receivedAt: number;
}

/** Estimate presentation time from the latest segment announcement. */
export function presentedCaptionTime(
  clock: CaptionClock | null,
  now: number,
): number | null {
  return clock ? clock.startMs + Math.max(0, now - clock.receivedAt) : null;
}

/** Rolling captions show at most this many cues at once. */
export const LIVE_CAPTION_MAX_LINES = 2;

// Cues older than this are pruned on every update, bounding memory on a
// long stream.
const LIVE_CAPTION_RETAIN_MS = 30000;

/**
 * Applies one place.stream.caption.defs#liveCue. Final text/start are
 * immutable; matching canonical finals may extend their end across GoPs.
 */
export function reduceLiveCaption(
  cues: Record<string, LiveCaption>,
  cue: place.stream.caption.defs.LiveCue,
  now: number,
): Record<string, LiveCaption> {
  const next: Record<string, LiveCaption> = {};
  for (const [id, c] of Object.entries(cues)) {
    if (
      now - c.updatedAt <= LIVE_CAPTION_RETAIN_MS ||
      c.endMs >= now - LIVE_CAPTION_RETAIN_MS
    ) {
      next[id] = c;
    }
  }
  const key = JSON.stringify([cue.track.id, cue.id]);
  const existing = next[key];
  if (existing?.final) {
    if (
      cue.track.origin === "canonical" &&
      cue.final &&
      cue.text === existing.text &&
      Date.parse(cue.startTime) === existing.startMs &&
      Date.parse(cue.endTime) > existing.endMs
    ) {
      next[key] = {
        ...existing,
        endMs: Date.parse(cue.endTime),
        updatedAt: now,
      };
    }
    return next;
  }
  const startMs = Date.parse(cue.startTime);
  const endMs = Date.parse(cue.endTime);
  next[key] = {
    id: cue.id,
    trackId: cue.track.id,
    text: cue.text,
    final: cue.final,
    startMs: Number.isFinite(startMs) ? startMs : now,
    endMs: Number.isFinite(endMs) ? endMs : now,
    updatedAt: now,
  };
  return next;
}

/** Cues covering the presented media wall clock, newest two, oldest first. */
export function activeLiveCaptions(
  cues: Record<string, LiveCaption>,
  trackId: string,
  now: number,
): LiveCaption[] {
  return Object.values(cues)
    .filter(
      (c) =>
        c.trackId === trackId &&
        c.text.trim() !== "" &&
        c.startMs <= now + 100 &&
        now < c.endMs,
    )
    .sort((a, b) => a.startMs - b.startMs)
    .slice(-LIVE_CAPTION_MAX_LINES);
}

/** OBS composites the streamer's live speech, independently of player latency. */
export function displayLiveCaptions(
  cues: Record<string, LiveCaption>,
  trackId: string,
  now: number,
): LiveCaption[] {
  const latest = Object.values(cues)
    .filter((c) => c.trackId === trackId && c.text.trim() !== "")
    .sort((a, b) => a.updatedAt - b.updatedAt || a.startMs - b.startMs)
    .at(-1);
  return latest &&
    now <
      latest.updatedAt +
        Math.max(LIVE_CAPTION_HOLD_MS, latest.endMs - latest.startMs)
    ? [latest]
    : [];
}

/** Follow the newest displayable canonical speech, else a sidecar. */
export function selectLiveCaptionTrack(
  options: CaptionTrackOption[],
  selectedId: string | null,
  cues: Record<string, LiveCaption>,
  now: number,
  display = activeLiveCaptions,
): CaptionTrackOption | null {
  if (selectedId)
    return options.find((track) => track.id === selectedId) ?? null;
  let selected: CaptionTrackOption | null = null;
  let latest: LiveCaption | undefined;
  for (const track of options) {
    const cue = display(cues, track.id, now).at(-1);
    if (!cue) continue;
    if (
      !selected ||
      (track.origin === "canonical" && selected.origin !== "canonical") ||
      (track.origin === selected.origin &&
        (cue.updatedAt > latest!.updatedAt ||
          (cue.updatedAt === latest!.updatedAt &&
            cue.startMs > latest!.startMs)))
    ) {
      selected = track;
      latest = cue;
    }
  }
  return selected;
}
