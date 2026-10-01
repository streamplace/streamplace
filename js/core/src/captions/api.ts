import type { CaptionTrackView } from "./tracks";

/** What a caption request is about: a live streamer or a video record. */
export type CaptionSubject = { streamer: string } | { video: string };

export type CaptionFormat = "vtt" | "srt" | "json";

/** One cue of getCaptions?format=json, offsets in ms from the video start. */
export interface TimedCaption {
  id?: string;
  startMs: number;
  endMs: number;
  text: string;
}

function subjectParams(subject: CaptionSubject): URLSearchParams {
  return new URLSearchParams(
    "video" in subject
      ? { video: subject.video }
      : { streamer: subject.streamer },
  );
}

/** URL of place.stream.caption.getCaptions, for downloads and fetches. */
export function captionsUrl(
  nodeUrl: string,
  subject: CaptionSubject,
  track: string,
  format: CaptionFormat,
): string {
  const params = subjectParams(subject);
  params.set("track", track);
  params.set("format", format);
  return `${nodeUrl}/xrpc/place.stream.caption.getCaptions?${params}`;
}

/**
 * Lists the caption tracks of a stream or video. A node without caption
 * support, or a subject without captions, yields no tracks rather than an
 * error: captions are optional everywhere.
 */
export async function fetchCaptionTracks(
  nodeUrl: string,
  subject: CaptionSubject,
  signal?: AbortSignal,
): Promise<CaptionTrackView[]> {
  const res = await fetch(
    `${nodeUrl}/xrpc/place.stream.caption.listTracks?${subjectParams(subject)}`,
    { signal },
  );
  if (!res.ok) return [];
  const body = await res.json();
  return Array.isArray(body?.tracks) ? body.tracks : [];
}

/**
 * Parses a getCaptions JSON document ({cues: [{id?, startMs, endMs,
 * text}]}) into cues sorted by start, dropping malformed entries.
 */
export function parseTimedCaptions(body: unknown): TimedCaption[] {
  if (!body || typeof body !== "object" || !("cues" in body)) return [];
  const cues = body.cues;
  if (!Array.isArray(cues)) return [];
  return cues
    .filter(
      (c): c is TimedCaption =>
        typeof c?.startMs === "number" &&
        typeof c?.endMs === "number" &&
        typeof c?.text === "string",
    )
    .sort((a, b) => a.startMs - b.startMs);
}

/** Fetches one track's cues as JSON; empty when unavailable. */
export async function fetchTimedCaptions(
  nodeUrl: string,
  subject: CaptionSubject,
  track: string,
  signal?: AbortSignal,
): Promise<TimedCaption[]> {
  const res = await fetch(captionsUrl(nodeUrl, subject, track, "json"), {
    signal,
  });
  if (!res.ok) return [];
  return parseTimedCaptions(await res.json());
}

/**
 * The cues covering playback position `ms`, oldest first. `cues` must be
 * sorted by start (parseTimedCaptions does that).
 */
export function timedCaptionsAt(
  cues: TimedCaption[],
  ms: number,
): TimedCaption[] {
  // Binary search for the first cue starting after ms; only cues before
  // it can cover ms.
  let lo = 0;
  let hi = cues.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (cues[mid].startMs <= ms) lo = mid + 1;
    else hi = mid;
  }
  // Any earlier cue can still overlap, even behind many short expired cues.
  const out: TimedCaption[] = [];
  for (let i = lo - 1; i >= 0 && out.length < 2; i--) {
    if (cues[i].endMs > ms) out.unshift(cues[i]);
  }
  return out;
}
