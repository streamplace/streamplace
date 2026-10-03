import { place } from "streamplace";

export type CaptionTrackView = place.stream.caption.defs.TrackView;

/**
 * A text track the playback element itself exposes: an HTMLVideoElement
 * TextTrack (hls.js or native HLS on the web) or an expo-video
 * SubtitleTrack (AVPlayer, ExoPlayer). `key` identifies it within that
 * element only.
 */
export interface ElementCaptionTrack {
  key: string;
  language: string;
  label: string;
}

/**
 * One entry of a player's caption menu. Server-known tracks carry their
 * trackView fields; `elementKey` is set when the playback element can
 * render (or hand us the cues of) this track itself.
 */
export interface CaptionTrackOption {
  id: string;
  language: string;
  label?: string;
  kind?: string;
  source?: string;
  origin?: string;
  elementKey?: string;
}

const ELEMENT_ID_PREFIX = "element:";

/** The primary language subtag, lowercased: "en-US" → "en". */
function primaryLanguage(tag: string): string {
  return tag.toLowerCase().split(/[-_]/)[0];
}

/** A separator- and case-insensitive form for exact locale-tag matching. */
function normalizedLanguage(tag: string): string {
  return tag.replace(/_/g, "-").toLowerCase();
}

/**
 * Merges the tracks a player knows about into one menu. Server tracks
 * (listTracks, websocket liveCue tracks) come first, deduplicated by id.
 * Each element track attaches to the first unmatched server track with
 * the same language, preferring one whose label also matches; element
 * tracks with no server counterpart become their own options.
 */
export function mergeCaptionTracks(
  known: CaptionTrackView[],
  element: ElementCaptionTrack[],
): CaptionTrackOption[] {
  const out: CaptionTrackOption[] = [];
  const seen = new Set<string>();
  for (const t of known) {
    if (seen.has(t.id)) continue;
    seen.add(t.id);
    out.push({
      id: t.id,
      language: t.language,
      label: t.label,
      kind: t.kind,
      source: t.source,
      origin: t.origin,
    });
  }
  for (const el of element) {
    const language = normalizedLanguage(el.language);
    const candidates = out.filter(
      (o) => !o.elementKey && normalizedLanguage(o.language) === language,
    );
    const match =
      candidates.find((o) => !!el.label && o.label === el.label) ??
      candidates[0];
    if (match) {
      match.elementKey = el.key;
    } else {
      out.push({
        id: `${ELEMENT_ID_PREFIX}${el.key}`,
        language: el.language,
        label: el.label || undefined,
        elementKey: el.key,
      });
    }
  }
  return out;
}

/**
 * Picks the track to show: the explicitly selected id when it is still
 * offered, else the first track in the viewer's preferred language
 * (exact tag, then primary subtag), else the first track.
 */
export function selectCaptionTrack(
  options: CaptionTrackOption[],
  selectedId: string | null,
  preferredLanguage: string | null,
): CaptionTrackOption | null {
  if (options.length === 0) return null;
  if (selectedId) {
    const picked = options.find((o) => o.id === selectedId);
    if (picked) return picked;
  }
  if (preferredLanguage) {
    const preferred = normalizedLanguage(preferredLanguage);
    const exact = options.find(
      (o) => normalizedLanguage(o.language) === preferred,
    );
    if (exact) return exact;
    const primary = primaryLanguage(preferredLanguage);
    const loose = options.find((o) => primaryLanguage(o.language) === primary);
    if (loose) return loose;
  }
  return options[0];
}

/**
 * The language's name in the display locale ("English", "español"),
 * falling back to the tag itself where Intl.DisplayNames is missing
 * (older Hermes) or the tag is invalid.
 */
export function captionLanguageName(
  language: string,
  displayLocale?: string,
): string {
  try {
    const names = new Intl.DisplayNames(
      displayLocale ? [displayLocale] : undefined,
      { type: "language" },
    );
    return names.of(language) ?? language;
  } catch {
    return language;
  }
}
