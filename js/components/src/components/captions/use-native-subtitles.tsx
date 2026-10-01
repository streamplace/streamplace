import type { SubtitleTrack, VideoPlayer } from "expo-video";
import { useEffect, useState } from "react";
import { usePlayerStore } from "../../player-store";
import { useCaptionSelection } from "./use-captions";

/**
 * Hands HLS subtitle renditions to expo-video, so AVPlayer and ExoPlayer
 * draw captions themselves and follow the viewer's OS caption style
 * (iOS Subtitles & Captioning, Android Caption preferences). Tracks the
 * player doesn't expose fall back to CaptionOverlay.
 */
export function useNativeSubtitles(player: VideoPlayer) {
  const setTracks = usePlayerStore((x) => x.setCaptionElementTracks);
  const setRenders = usePlayerStore((x) => x.setCaptionElementRenders);
  const { active } = useCaptionSelection();
  const activeKey = active?.elementKey ?? null;
  const [available, setAvailable] = useState<SubtitleTrack[]>(
    () => player.availableSubtitleTracks,
  );

  useEffect(() => {
    setAvailable(player.availableSubtitleTracks);
    const sub = player.addListener(
      "availableSubtitleTracksChange",
      ({ availableSubtitleTracks }) => setAvailable(availableSubtitleTracks),
    );
    return () => sub.remove();
  }, [player]);

  useEffect(() => {
    // `name` is the rendition NAME from the playlist on both platforms;
    // `label` is a localized display name (AVMediaSelectionOption
    // displayName on iOS).
    setTracks(
      available.map((t, i) => ({
        key: String(i),
        language: t.language,
        label: t.name || t.label,
      })),
    );
  }, [available, setTracks]);

  useEffect(() => {
    // Assign the player's own track object: iOS matches it by label and
    // language, Android by id.
    const track =
      activeKey === null ? null : (available[Number(activeKey)] ?? null);
    player.subtitleTrack = track;
    setRenders(track !== null);
  }, [player, available, activeKey, setRenders]);

  useEffect(
    () => () => {
      setTracks([]);
      setRenders(false);
    },
    [setTracks, setRenders],
  );
}
