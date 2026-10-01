import { place } from "streamplace";

/** A live cue as the player keeps it. */
export interface LiveCaption {
  id: string;
  trackId: string;
  text: string;
  final: boolean;
  /** Cue start on the segment wall clock, ms since the epoch. */
  startMs: number;
  /** Local time of the last revision, ms since the epoch. */
  updatedAt: number;
}

/**
 * How long a cue stays on screen after its last revision. Live cues
 * render as they arrive (WebRTC plays within a second of the encoder),
 * so this is the reading time a line gets once speech moves on.
 */
export const LIVE_CAPTION_HOLD_MS = 5000;

/** Rolling captions show at most this many cues at once. */
export const LIVE_CAPTION_MAX_LINES = 2;

// Cues older than this are pruned on every update, bounding memory on a
// long stream.
const LIVE_CAPTION_RETAIN_MS = 30000;

/**
 * Applies one place.stream.caption.defs#liveCue. A cue replaces an earlier
 * version with the same track and cue ids, unless that version was final
 * (finals never change, so a late interim must not overwrite one).
 */
export function reduceLiveCaption(
  cues: Record<string, LiveCaption>,
  cue: place.stream.caption.defs.LiveCue,
  now: number,
): Record<string, LiveCaption> {
  const next: Record<string, LiveCaption> = {};
  for (const [id, c] of Object.entries(cues)) {
    if (now - c.updatedAt <= LIVE_CAPTION_RETAIN_MS) next[id] = c;
  }
  const key = JSON.stringify([cue.track.id, cue.id]);
  const existing = next[key];
  if (existing?.final) return next;
  const startMs = Date.parse(cue.startTime);
  next[key] = {
    id: cue.id,
    trackId: cue.track.id,
    text: cue.text,
    final: cue.final,
    startMs: Number.isFinite(startMs) ? startMs : now,
    updatedAt: now,
  };
  return next;
}

/**
 * The cues of one track to show now: revised within the hold window,
 * non-empty, oldest first, at most LIVE_CAPTION_MAX_LINES.
 */
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
        now - c.updatedAt <= LIVE_CAPTION_HOLD_MS,
    )
    .sort((a, b) => a.startMs - b.startMs)
    .slice(-LIVE_CAPTION_MAX_LINES);
}
