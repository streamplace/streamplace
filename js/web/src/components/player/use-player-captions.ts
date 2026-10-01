import { useStore as useAppStore } from "@/lib/store";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  activeLiveCaptions,
  type CaptionTrackOption,
  type CaptionTrackView,
  type ElementCaptionTrack,
  fetchCaptionTracks,
  fetchTimedCaptions,
  type LivestreamStore,
  makeLivestreamStore,
  mergeCaptionTracks,
  presentedCaptionTime,
  selectCaptionTrack,
  type TextTrackWatcher,
  type TimedCaption,
  timedCaptionsAt,
  watchTextTracks,
} from "@streamplace/core";
import {
  type RefObject,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useStore } from "zustand";

/**
 * Where a player's captions come from: a live streamer (DID or handle),
 * with the livestream store that carries its websocket caption cues, or
 * a place.stream.video record.
 */
export type PlayerCaptionSource =
  | { streamer: string; store?: LivestreamStore }
  | { video: string };

export interface PlayerCaptions {
  tracks: CaptionTrackOption[];
  /** The track the menu shows as chosen, even while captions are off. */
  track: CaptionTrackOption | null;
  enabled: boolean;
  /** Caption lines to draw now. */
  lines: string[];
  /** Picks a track (turning captions on), or turns them off with null. */
  select: (track: CaptionTrackOption | null) => void;
  toggle: () => void;
}

const NO_LINES: string[] = [];
// Stands in for the livestream store on VODs, so hooks run unconditionally.
const emptyLivestreamStore = makeLivestreamStore();

/**
 * Caption state for one <Player>: the track menu (server tracks, live cue
 * tracks, and the video element's HLS text tracks), the viewer's
 * persisted on/off and language, and the lines to draw. Lines come from
 * the selected HLS text track when the element has it, else from live
 * websocket cues, else from the video's JSON captions by play position.
 */
export function usePlayerCaptions(
  videoRef: RefObject<HTMLVideoElement | null>,
  source: PlayerCaptionSource | undefined,
  active: boolean,
): PlayerCaptions {
  const enabled = useAppStore((s) => s.captionsEnabled);
  const language = useAppStore((s) => s.captionLanguage);
  const setEnabled = useAppStore((s) => s.setCaptionsEnabled);
  const setLanguage = useAppStore((s) => s.setCaptionLanguage);

  const subject = source
    ? "video" in source
      ? { video: source.video }
      : { streamer: source.streamer }
    : null;
  const subjectKey = subject ? JSON.stringify(subject) : "";
  const isLive = !!subject && "streamer" in subject;
  const liveStore =
    source && "store" in source && source.store
      ? source.store
      : emptyLivestreamStore;
  const liveTracks = useStore(liveStore, (s) => s.captionTracks);
  const liveCues = useStore(liveStore, (s) => s.liveCaptions);
  const captionClock = useStore(liveStore, (s) => s.captionClock);
  const liveSession = useStore(liveStore, (s) =>
    s.livestream
      ? `${s.livestream.uri}:${s.livestream.record.endedAt ?? ""}`
      : null,
  );

  const [serverTracks, setServerTracks] = useState<CaptionTrackView[]>([]);
  useEffect(() => {
    setServerTracks([]);
    if (!subjectKey) return;
    const controller = new AbortController();
    fetchCaptionTracks(
      getStreamplaceUrl(),
      JSON.parse(subjectKey),
      controller.signal,
    )
      .then(setServerTracks)
      .catch((err) => {
        if (!controller.signal.aborted) {
          console.error("[captions] failed to list tracks", err);
        }
      });
    return () => controller.abort();
  }, [subjectKey, liveSession]);

  // The <video>'s own text tracks, from hls.js or Safari.
  const [elementTracks, setElementTracks] = useState<ElementCaptionTrack[]>([]);
  const [elementLines, setElementLines] = useState<string[]>(NO_LINES);
  const watcher = useRef<TextTrackWatcher | null>(null);
  const activeKeyRef = useRef<string | null>(null);
  useEffect(() => {
    const video = videoRef.current;
    if (!active || !video) return;
    const w = watchTextTracks(video, setElementTracks, setElementLines);
    w.setActive(activeKeyRef.current);
    watcher.current = w;
    return () => {
      w.dispose();
      watcher.current = null;
      setElementTracks([]);
      setElementLines(NO_LINES);
    };
  }, [active, videoRef]);

  const tracks = useMemo(
    () =>
      mergeCaptionTracks(
        isLive ? [...serverTracks, ...liveTracks] : serverTracks,
        elementTracks,
      ),
    [isLive, serverTracks, liveTracks, elementTracks],
  );
  const [trackId, setTrackId] = useState<string | null>(null);
  const track = selectCaptionTrack(tracks, trackId, language);
  const shown = enabled ? track : null;
  const activeKey = shown?.elementKey ?? null;
  activeKeyRef.current = activeKey;
  useEffect(() => {
    watcher.current?.setActive(activeKey);
  }, [activeKey]);

  // Live cues follow the segment presentation clock, including queued future cues.
  const liveTrackId = shown && !shown.elementKey && isLive ? shown.id : null;
  const [tick, setTick] = useState(0);
  const liveLines = useMemo(
    () => {
      const presented = presentedCaptionTime(captionClock, Date.now());
      return liveTrackId && presented !== null
        ? activeLiveCaptions(liveCues, liveTrackId, presented).map(
            (c) => c.text,
          )
        : NO_LINES;
    },
    // tick only forces a re-evaluation against the current time.
    [liveCues, captionClock, liveTrackId, tick],
  );
  useEffect(() => {
    if (!liveTrackId || !captionClock) return;
    const timer = setInterval(() => setTick((n) => n + 1), 250);
    return () => clearInterval(timer);
  }, [liveTrackId, captionClock]);

  // VOD cues as JSON, looked up by the element's play position.
  const vodTrackId =
    shown && !shown.elementKey && subject && "video" in subject
      ? shown.id
      : null;
  const [timed, setTimed] = useState<TimedCaption[]>([]);
  useEffect(() => {
    setTimed([]);
    if (!vodTrackId || !subjectKey) return;
    const controller = new AbortController();
    fetchTimedCaptions(
      getStreamplaceUrl(),
      JSON.parse(subjectKey),
      vodTrackId,
      controller.signal,
    )
      .then(setTimed)
      .catch((err) => {
        if (!controller.signal.aborted) {
          console.error("[captions] failed to load captions", err);
        }
      });
    return () => controller.abort();
  }, [vodTrackId, subjectKey]);
  const [timedText, setTimedText] = useState("");
  useEffect(() => {
    const video = videoRef.current;
    if (!video || timed.length === 0) {
      setTimedText("");
      return;
    }
    const update = () =>
      setTimedText(
        timedCaptionsAt(timed, video.currentTime * 1000)
          .map((c) => c.text)
          .join("\n"),
      );
    update();
    video.addEventListener("timeupdate", update);
    video.addEventListener("seeked", update);
    return () => {
      video.removeEventListener("timeupdate", update);
      video.removeEventListener("seeked", update);
    };
  }, [timed, videoRef, active]);
  const timedLines = useMemo(
    () => (timedText ? timedText.split("\n") : NO_LINES),
    [timedText],
  );

  const select = useCallback(
    (next: CaptionTrackOption | null) => {
      if (!next) {
        setEnabled(false);
        return;
      }
      setTrackId(next.id);
      setLanguage(next.language);
      setEnabled(true);
    },
    [setEnabled, setLanguage],
  );
  const toggle = useCallback(() => setEnabled(!enabled), [setEnabled, enabled]);

  const lines = !shown
    ? NO_LINES
    : shown.elementKey
      ? elementLines
      : isLive
        ? liveLines
        : timedLines;

  return { tracks, track, enabled, lines, select, toggle };
}
