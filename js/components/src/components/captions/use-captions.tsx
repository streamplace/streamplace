import {
  activeLiveCaptions,
  captionLanguageName,
  CaptionTrackOption,
  fetchCaptionTracks,
  fetchTimedCaptions,
  mergeCaptionTracks,
  selectCaptionTrack,
  TimedCaption,
  timedCaptionsAt,
} from "@streamplace/core";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLivestreamStoreOptional } from "../../livestream-store";
import { usePlayerStore } from "../../player-store";
import {
  useCaptionLanguage,
  useCaptionsEnabled,
  useSetCaptionLanguage,
  useSetCaptionsEnabled,
  useStreamplaceStore,
} from "../../streamplace-store";

const NO_TRACKS: never[] = [];

/**
 * Every caption track this player can show: the server's tracks
 * (listTracks, plus tracks seen in live cues) merged with the text tracks
 * of the playback element.
 */
export function useCaptionTracks(): CaptionTrackOption[] {
  const mode = usePlayerStore((x) => x.mode);
  const server = usePlayerStore((x) => x.captionServerTracks);
  const element = usePlayerStore((x) => x.captionElementTracks);
  const live = useLivestreamStoreOptional((x) => x.captionTracks);
  return useMemo(
    () =>
      mergeCaptionTracks(
        mode === "live" ? [...server, ...live] : server,
        element,
      ),
    [mode, server, live, element],
  );
}

export interface CaptionSelection {
  tracks: CaptionTrackOption[];
  /** The track the menu shows as chosen, even while captions are off. */
  track: CaptionTrackOption | null;
  enabled: boolean;
  /** The track to render now: `track` when captions are on. */
  active: CaptionTrackOption | null;
}

export function useCaptionSelection(): CaptionSelection {
  const tracks = useCaptionTracks();
  const trackId = usePlayerStore((x) => x.captionTrackId);
  const language = useCaptionLanguage();
  const enabled = useCaptionsEnabled();
  return useMemo(() => {
    const track = selectCaptionTrack(tracks, trackId, language);
    return { tracks, track, enabled, active: enabled ? track : null };
  }, [tracks, trackId, language, enabled]);
}

/**
 * Chooses a track (turning captions on and remembering its language for
 * other players), or turns captions off with null.
 */
export function useSetCaptionTrack(): (
  track: CaptionTrackOption | null,
) => void {
  const setTrackId = usePlayerStore((x) => x.setCaptionTrackId);
  const setLanguage = useSetCaptionLanguage();
  const setEnabled = useSetCaptionsEnabled();
  return useCallback(
    (track) => {
      if (!track) {
        setEnabled(false);
        return;
      }
      setTrackId(track.id);
      setLanguage(track.language);
      setEnabled(true);
    },
    [setTrackId, setLanguage, setEnabled],
  );
}

/** Turns captions on (the selected or preferred track) or off; the `c` key. */
export function useToggleCaptions(): () => void {
  const { track, enabled } = useCaptionSelection();
  const setTrack = useSetCaptionTrack();
  return useCallback(
    () => setTrack(enabled ? null : track),
    [enabled, track, setTrack],
  );
}

/** Menu label for a track: its language, marked when auto-generated. */
export function useCaptionTrackLabel(): (track: CaptionTrackOption) => string {
  const { t, i18n } = useTranslation();
  return useCallback(
    (track) => {
      const name = captionLanguageName(track.language, i18n.language);
      const base = name === track.language && track.label ? track.label : name;
      return track.source === "auto"
        ? t("player-captions-track-auto", { language: base })
        : base;
    },
    [t, i18n.language],
  );
}

/**
 * Loads the server's caption tracks for the player's stream or video.
 * Mount once per player (CaptionOverlay does).
 */
export function useLoadCaptionTracks() {
  const mode = usePlayerStore((x) => x.mode);
  const src = usePlayerStore((x) => x.src);
  const setServerTracks = usePlayerStore((x) => x.setCaptionServerTracks);
  const url = useStreamplaceStore((x) => x.url);
  useEffect(() => {
    setServerTracks(NO_TRACKS);
    if (!src) return;
    const controller = new AbortController();
    fetchCaptionTracks(
      url,
      mode === "vod" ? { video: src } : { streamer: src },
      controller.signal,
    )
      .then(setServerTracks)
      .catch((err) => {
        if (!controller.signal.aborted) {
          console.error("failed to list caption tracks", err);
        }
      });
    return () => controller.abort();
  }, [url, mode, src, setServerTracks]);
}

// Live cues of a track. While lines are on screen, re-evaluate every
// second so they leave once stale even when no new cue arrives.
function useLiveCaptionLines(trackId: string | null): string[] {
  const cues = useLivestreamStoreOptional((x) => x.liveCaptions);
  const [tick, setTick] = useState(0);
  const active = useMemo(
    () => (trackId ? activeLiveCaptions(cues, trackId, Date.now()) : []),
    // tick only forces a re-evaluation against the current time.
    [cues, trackId, tick],
  );
  const showing = active.length > 0;
  useEffect(() => {
    if (!showing) return;
    const timer = setInterval(() => setTick((n) => n + 1), 1000);
    return () => clearInterval(timer);
  }, [showing]);
  return useMemo(() => active.map((c) => c.text), [active]);
}

// VOD cues fetched as JSON and looked up by play position.
function useTimedCaptionLines(trackId: string | null): string[] {
  const src = usePlayerStore((x) => x.src);
  const playTime = usePlayerStore((x) => x.playTime);
  const url = useStreamplaceStore((x) => x.url);
  const [cues, setCues] = useState<TimedCaption[]>([]);
  useEffect(() => {
    setCues([]);
    if (!trackId || !src) return;
    const controller = new AbortController();
    fetchTimedCaptions(url, { video: src }, trackId, controller.signal)
      .then(setCues)
      .catch((err) => {
        if (!controller.signal.aborted) {
          console.error("failed to load captions", err);
        }
      });
    return () => controller.abort();
  }, [url, src, trackId]);
  const shown = timedCaptionsAt(cues, playTime * 1000);
  const key = shown.map((c) => c.text).join("\n");
  return useMemo(() => (key ? key.split("\n") : []), [key]);
}

/**
 * The caption lines to draw over the video right now. Empty when
 * captions are off, or when the playback element renders the track
 * natively.
 */
export function useCaptionLines(): string[] {
  const { active } = useCaptionSelection();
  const mode = usePlayerStore((x) => x.mode);
  const elementRenders = usePlayerStore((x) => x.captionElementRenders);
  const elementLines = usePlayerStore((x) => x.captionElementLines);
  const fromElement = !!active?.elementKey;
  const serverTrack = active && !fromElement ? active.id : null;
  const live = useLiveCaptionLines(mode === "live" ? serverTrack : null);
  const timed = useTimedCaptionLines(mode === "vod" ? serverTrack : null);
  if (!active) return NO_TRACKS;
  if (fromElement) return elementRenders ? NO_TRACKS : elementLines;
  return mode === "live" ? live : timed;
}
