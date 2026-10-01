import type { ElementCaptionTrack } from "./tracks";

// WebVTT cue text may carry markup (<v Speaker>, <i>, <c.class>); caption
// overlays draw plain lines.
function cueLines(cues: TextTrackCueList | null): string[] {
  if (!cues) return [];
  const lines: string[] = [];
  for (const cue of Array.from(cues)) {
    const payload =
      "text" in cue && typeof cue.text === "string" ? cue.text : "";
    const text =
      "getCueAsHTML" in cue && typeof cue.getCueAsHTML === "function"
        ? ((cue.getCueAsHTML() as DocumentFragment).textContent ?? "")
        : payload
            .replace(/<[^>]*>/g, "")
            .replace(
              /&(amp|lt|gt|nbsp|lrm|rlm);/g,
              (_, name: string) =>
                ({
                  amp: "&",
                  lt: "<",
                  gt: ">",
                  nbsp: "\u00a0",
                  lrm: "\u200e",
                  rlm: "\u200f",
                })[name] ?? "",
            );
    for (const line of text.split("\n")) {
      if (line.trim() !== "") lines.push(line);
    }
  }
  return lines;
}

export interface TextTrackWatcher {
  /** Shows the track with this key (from onTracks), or none. */
  setActive: (key: string | null) => void;
  dispose: () => void;
}

/**
 * Bridges a <video>'s subtitle and caption TextTracks (hls.js subtitle
 * renditions, Safari's native HLS) to a caption overlay. The active track
 * runs in "hidden" mode, so the browser loads and times its cues but
 * draws nothing, and its current cue lines go to `onLines`; every other
 * track is disabled. `onTracks` reports the tracks as they come and go.
 */
export function watchTextTracks(
  video: HTMLVideoElement,
  onTracks: (tracks: ElementCaptionTrack[]) => void,
  onLines: (lines: string[]) => void,
): TextTrackWatcher {
  const list = video.textTracks;
  const keys = new WeakMap<TextTrack, string>();
  const subscribed = new Set<TextTrack>();
  let nextKey = 0;
  let activeKey: string | null = null;
  let lastLines = "";

  const keyOf = (track: TextTrack) => {
    let key = keys.get(track);
    if (key === undefined) {
      key = String(nextKey++);
      keys.set(track, key);
    }
    return key;
  };
  const captionTracks = () =>
    Array.from(list).filter(
      (t) => t.kind === "subtitles" || t.kind === "captions",
    );

  const updateLines = () => {
    const track = captionTracks().find((t) => keyOf(t) === activeKey);
    const lines = track ? cueLines(track.activeCues) : [];
    const joined = lines.join("\n");
    if (joined === lastLines) return;
    lastLines = joined;
    onLines(lines);
  };

  const apply = () => {
    const current = captionTracks();
    for (const track of subscribed) {
      if (!current.includes(track)) {
        track.removeEventListener("cuechange", updateLines);
        subscribed.delete(track);
      }
    }
    for (const track of current) {
      const mode = keyOf(track) === activeKey ? "hidden" : "disabled";
      if (track.mode !== mode) track.mode = mode;
      // Re-adding the same listener is a no-op.
      track.addEventListener("cuechange", updateLines);
      subscribed.add(track);
    }
    updateLines();
  };

  const onListChanged = () => {
    onTracks(
      captionTracks().map((t) => ({
        key: keyOf(t),
        language: t.language,
        label: t.label,
      })),
    );
    apply();
  };

  onListChanged();
  list.addEventListener("addtrack", onListChanged);
  list.addEventListener("removetrack", onListChanged);
  // hls.js and Safari switch modes on their own (default tracks, the
  // viewer's OS caption preference); put ours back.
  list.addEventListener("change", apply);

  return {
    setActive: (key) => {
      activeKey = key;
      apply();
    },
    dispose: () => {
      list.removeEventListener("addtrack", onListChanged);
      list.removeEventListener("removetrack", onListChanged);
      list.removeEventListener("change", apply);
      for (const track of subscribed) {
        track.removeEventListener("cuechange", updateLines);
      }
      subscribed.clear();
    },
  };
}
