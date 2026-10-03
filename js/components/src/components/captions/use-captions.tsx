import {
  CaptionTrackOption,
  fetchCaptionTracks,
  fetchTimedCaptions,
  liveCaptionLines,
  mergeCaptionTracks,
  selectCaptionTrack,
  selectLiveCaptionTrack,
  TimedCaption,
  timedCaptionsAt,
  useCaptionTime,
} from "@streamplace/core";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useLivestreamStoreOptional } from "../../livestream-store";
import { PlayerStatus, usePlayerStore } from "../../player-store";
import {
  useCaptionLanguage,
  useCaptionPrefs,
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
  presented: number | null;
}

export function useCaptionSelection(): CaptionSelection {
  const tracks = useCaptionTracks();
  const trackId = usePlayerStore((x) => x.captionTrackId);
  const language = useCaptionLanguage();
  const enabled = useCaptionsEnabled();
  const mode = usePlayerStore((x) => x.mode);
  const paused = usePlayerStore((x) => x.status === PlayerStatus.PAUSE);
  const cues = useLivestreamStoreOptional((x) => x.liveCaptions);
  const clock = useLivestreamStoreOptional((x) => x.captionClock);
  const presented = useCaptionTime(mode === "live" ? clock : null, paused);
  return useMemo(() => {
    const track =
      (mode === "live" && presented !== null
        ? selectLiveCaptionTrack(tracks, trackId, cues, presented)
        : null) ?? selectCaptionTrack(tracks, trackId, language);
    return {
      tracks,
      track,
      enabled,
      active: enabled ? track : null,
      presented,
    };
  }, [tracks, trackId, language, enabled, mode, cues, presented]);
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
  const enabled = useCaptionsEnabled();
  const setEnabled = useSetCaptionsEnabled();
  return useCallback(() => setEnabled(!enabled), [enabled, setEnabled]);
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
  const session = useLivestreamStoreOptional((x) =>
    x.livestream
      ? `${x.livestream.uri}:${x.livestream.record.endedAt ?? ""}`
      : null,
  );
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
  }, [url, mode, src, session, setServerTracks]);
}

// Live cues follow the segment presentation clock, including queued future cues.
function useLiveCaptionLines(
  track: CaptionTrackOption | null,
  presented: number | null,
): string[] {
  const cues = useLivestreamStoreOptional((x) => x.liveCaptions);
  const size = useCaptionPrefs().size;
  return useMemo(
    () =>
      track && presented !== null
        ? liveCaptionLines(cues, track, presented, size)
        : [],
    [cues, track, presented, size],
  );
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
  const { active, presented } = useCaptionSelection();
  const mode = usePlayerStore((x) => x.mode);
  const elementRenders = usePlayerStore((x) => x.captionElementRenders);
  const elementLines = usePlayerStore((x) => x.captionElementLines);
  const fromElement = !!active?.elementKey;
  const serverTrack = active && !fromElement ? active : null;
  const live = useLiveCaptionLines(
    mode === "live" ? serverTrack : null,
    presented,
  );
  const timed = useTimedCaptionLines(
    mode === "vod" ? (serverTrack?.id ?? null) : null,
  );
  if (!active) return NO_TRACKS;
  if (fromElement) return elementRenders ? NO_TRACKS : elementLines;
  return mode === "live" ? live : timed;
}
