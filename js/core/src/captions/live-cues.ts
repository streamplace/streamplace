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
  /** How far the cue was moved to show after arriving late; see reduceLiveCaption. */
  shiftMs: number;
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

/**
 * How long the newest cue of a track stays up past its end while no later
 * cue is known: a canonical cue's end is provisional until the next
 * segment's text arrives, and recognized speech pauses between batches.
 */
export const LIVE_CAPTION_LINGER_MS = 2000;

/** Recognized speech rolls up in rows of this many columns at the default size. */
const LIVE_CAPTION_ROW_COLUMNS = 42;

/** Maximum retained cues per track, independent of server-side limits. */
export const LIVE_CAPTION_MAX_CUES_PER_TRACK = 32;

// History/lookahead horizon; the count cap also bounds untrusted distant ends.
const LIVE_CAPTION_RETAIN_MS = 30000;

/**
 * Applies one place.stream.caption.defs#liveCue. Final text/start are
 * immutable; matching canonical finals may extend their end across GoPs.
 */
export function reduceLiveCaption(
  cues: Record<string, LiveCaption>,
  cue: place.stream.caption.defs.LiveCue,
  now: number,
  presentationMs = now,
): Record<string, LiveCaption> {
  let next = cues;
  for (const id in cues) {
    const c = cues[id];
    if (
      c.startMs > presentationMs + LIVE_CAPTION_RETAIN_MS ||
      (now - c.updatedAt > LIVE_CAPTION_RETAIN_MS &&
        c.endMs < presentationMs - LIVE_CAPTION_RETAIN_MS)
    ) {
      if (next === cues) next = { ...cues };
      delete next[id];
    }
  }
  const parsedStart = Date.parse(cue.startTime);
  const startMs = Number.isFinite(parsedStart) ? parsedStart : now;
  const endMs = Date.parse(cue.endTime);
  if (startMs > presentationMs + LIVE_CAPTION_RETAIN_MS) return next;
  const key = JSON.stringify([cue.track.id, cue.id]);
  const existing = next[key];
  if (existing?.final) {
    if (
      cue.track.origin === "canonical" &&
      cue.final &&
      cue.text === existing.text &&
      parsedStart === existing.startMs &&
      endMs > existing.endMs
    ) {
      if (next === cues) next = { ...cues };
      next[key] = {
        ...existing,
        endMs,
        updatedAt: now,
      };
    }
    return next;
  }
  // Node and directly pushed captions keep their speech times, which a
  // low-latency player has already presented by the time they are published:
  // such a late cue shows from the presentation time for its own duration,
  // and its revisions stay where it appeared. Canonical cues were placed in
  // the stream already.
  const shiftMs =
    existing?.shiftMs ??
    (cue.track.origin === "canonical"
      ? 0
      : Math.max(0, presentationMs - startMs));
  const shownStart = startMs + shiftMs;
  if (!existing) {
    const keys: string[] = [];
    for (const id in next) {
      if (next[id].trackId === cue.track.id) keys.push(id);
    }
    if (keys.length >= LIVE_CAPTION_MAX_CUES_PER_TRACK) {
      keys.push(key);
      keys.sort(
        (a, b) =>
          (b === key ? shownStart : next[b].startMs) -
            (a === key ? shownStart : next[a].startMs) ||
          (b === key ? now : next[b].updatedAt) -
            (a === key ? now : next[a].updatedAt),
      );
      for (let i = LIVE_CAPTION_MAX_CUES_PER_TRACK; i < keys.length; i++) {
        if (keys[i] === key) continue;
        if (next === cues) next = { ...cues };
        delete next[keys[i]];
      }
      if (keys.indexOf(key) >= LIVE_CAPTION_MAX_CUES_PER_TRACK) return next;
    }
  }
  if (next === cues) next = { ...cues };
  next[key] = {
    id: cue.id,
    trackId: cue.track.id,
    text: cue.text,
    final: cue.final,
    startMs: shownStart,
    endMs: (Number.isFinite(endMs) ? endMs : now) + shiftMs,
    shiftMs,
    updatedAt: now,
  };
  return next;
}

/**
 * Cues covering the presented media wall clock, newest two, oldest first.
 * The newest cue lingers past its end until a later cue is known, which
 * keeps the gaps a later cue does define.
 */
export function activeLiveCaptions(
  cues: Record<string, LiveCaption>,
  trackId: string,
  now: number,
): LiveCaption[] {
  const track = trackCues(cues, trackId);
  const last = track.at(-1);
  return track
    .filter(
      (c) =>
        c.startMs <= now + 100 &&
        now < c.endMs + (c === last ? LIVE_CAPTION_LINGER_MS : 0),
    )
    .slice(-LIVE_CAPTION_MAX_LINES);
}

// The track's cues with text, by start.
function trackCues(
  cues: Record<string, LiveCaption>,
  trackId: string,
): LiveCaption[] {
  return Object.values(cues)
    .filter((c) => c.trackId === trackId && c.text.trim() !== "")
    .sort((a, b) => a.startMs - b.startMs);
}

/**
 * The lines to draw for a live track: recognized speech rolls up (see
 * rolledLiveCaptions); other captions keep the lines their author cued.
 * `size` is the viewer's caption size, percent.
 */
export function liveCaptionLines(
  cues: Record<string, LiveCaption>,
  track: CaptionTrackOption,
  now: number,
  size = 100,
): string[] {
  const language = track.language.split("-")[0].toLowerCase();
  if (track.source !== "auto" || UNSPACED_LANGUAGES.has(language)) {
    return activeLiveCaptions(cues, track.id, now).map((c) => c.text);
  }
  return rolledLiveCaptions(
    cues,
    track.id,
    now,
    Math.max(12, Math.round((LIVE_CAPTION_ROW_COLUMNS * 100) / size)),
  );
}

// Scripts written without spaces between words, whose batches can't be
// joined with one.
const UNSPACED_LANGUAGES = new Set(["ja", "zh", "yue", "th", "lo", "km", "my"]);
const SENTENCE_END = /[.!?…。！？]["'”’)\]]*$/u;
const WIDE =
  /[\u1100-\u115f\u2e80-\ua4cf\uac00-\ud7a3\uf900-\ufaff\ufe30-\ufe4f\uff00-\uff60\uffe0-\uffe6]/u;

function columns(text: string): number {
  let n = 0;
  for (const ch of text) n += WIDE.test(ch) ? 2 : 1;
  return n;
}

/**
 * Recognized speech as roll-up rows: consecutive cues (agreed batches of
 * arbitrary length) run together and wrap at `rowColumns`, so a short batch
 * joins the row before it instead of standing alone, and the rows stay up
 * across the pauses between batches. Each sentence starts a row, which also
 * keeps rows from reflowing as old cues expire. Shows every row of the
 * newest cue, and at least the last LIVE_CAPTION_MAX_LINES rows.
 */
function rolledLiveCaptions(
  cues: Record<string, LiveCaption>,
  trackId: string,
  now: number,
  rowColumns = LIVE_CAPTION_ROW_COLUMNS,
): string[] {
  const started = trackCues(cues, trackId).filter(
    (c) => c.startMs <= now + 100,
  );
  const newest = started.at(-1);
  if (!newest || now >= newest.endMs + LIVE_CAPTION_LINGER_MS) return [];
  // The rows run back to where the screen last cleared.
  let first = started.length - 1;
  while (
    first > 0 &&
    started[first].startMs <= started[first - 1].endMs + LIVE_CAPTION_LINGER_MS
  ) {
    first--;
  }
  const rows: string[] = [];
  let newestRow = -1;
  let sentenceEnded = true;
  for (let i = first; i < started.length; i++) {
    for (const word of started[i].text.trim().split(/\s+/)) {
      const last = rows.length - 1;
      if (
        !sentenceEnded &&
        columns(rows[last]) + 1 + columns(word) <= rowColumns
      ) {
        rows[last] += ` ${word}`;
      } else {
        rows.push(word);
      }
      if (i === started.length - 1 && newestRow < 0) {
        newestRow = rows.length - 1;
      }
      sentenceEnded = SENTENCE_END.test(word);
    }
  }
  return rows.slice(
    Math.max(0, Math.min(newestRow, rows.length - LIVE_CAPTION_MAX_LINES)),
  );
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
