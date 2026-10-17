import { useEffect } from "react";
import { Platform } from "react-native";
import { PlayerStatus, usePlayerStore } from "../../../player-store";
import { vodFrameRateForQuality } from "../vod-levels";

const SHORTCUTS_BLOCKED_BY_CONTROLS =
  'input, textarea, select, a, [contenteditable], [role="textbox"], [role="slider"], [role="menu"], [role="dialog"]';
const PLAYER_BUTTON = 'button, [role="button"]';

export function PlayerKeyboardShortcuts() {
  const mode = usePlayerStore((state) => state.mode);
  const status = usePlayerStore((state) => state.status);
  const playbackRate = usePlayerStore((state) => state.playbackRate);
  const setPlaybackRate = usePlayerStore((state) => state.setPlaybackRate);
  const togglePlayPause = usePlayerStore((state) => state.togglePlayPause);
  const showFeedback = usePlayerStore((state) => state.showFeedback);
  const playTime = usePlayerStore((state) => state.playTime);
  const duration = usePlayerStore((state) => state.duration);
  const videoRef = usePlayerStore((state) => state.videoRef);
  const seekTo = usePlayerStore((state) => state.seekTo);
  const selectedRendition = usePlayerStore((state) => state.selectedRendition);
  const vodLevels = usePlayerStore((state) => state.vodLevels);
  const playingVODRendition = usePlayerStore(
    (state) => state.playingVODRendition,
  );

  useEffect(() => {
    if (Platform.OS !== "web" || typeof window === "undefined") return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (
        event.defaultPrevented ||
        event.ctrlKey ||
        event.metaKey ||
        event.altKey ||
        event.isComposing
      ) {
        return;
      }

      const target = event.target as HTMLElement | null;
      if (
        target?.isContentEditable ||
        target?.closest?.(SHORTCUTS_BLOCKED_BY_CONTROLS)
      ) {
        return;
      }

      const key = event.key.toLowerCase();
      const isPlayerButton = target?.closest?.(PLAYER_BUTTON);
      if (key === " " || key === "spacebar" || key === "k") {
        // Let a focused button keep its native Space activation behavior.
        if (isPlayerButton && key !== "k") return;
        event.preventDefault();
        if (!event.repeat) togglePlayPause();
        return;
      }

      if (mode !== "vod") return;

      if (
        isPlayerButton &&
        key !== "," &&
        key !== "." &&
        key !== "<" &&
        key !== ">"
      ) {
        return;
      }

      if (key === "<" || key === ">") {
        event.preventDefault();
        const nextRate = Math.max(
          0.25,
          Math.min(2, playbackRate + (key === ">" ? 0.25 : -0.25)),
        );
        setPlaybackRate(nextRate);
        showFeedback({ type: "speed", playbackRate: nextRate });
        return;
      }

      if (
        (key === "," || key === ".") &&
        selectedRendition !== "audio" &&
        status !== PlayerStatus.PLAYING &&
        Number.isFinite(duration) &&
        duration > 0
      ) {
        const currentTime =
          videoRef && typeof videoRef === "object" && "current" in videoRef
            ? (videoRef.current?.currentTime ?? playTime)
            : playTime;
        const frameRate = vodFrameRateForQuality(
          vodLevels,
          selectedRendition,
          playingVODRendition,
        );
        const direction = key === "." ? "forward" : "back";
        event.preventDefault();
        seekTo(
          Math.max(
            0,
            Math.min(
              duration,
              currentTime + (direction === "forward" ? 1 : -1) / frameRate,
            ),
          ),
        );
        showFeedback({ type: "frame", direction });
        return;
      }

      if (
        (key === "arrowleft" || key === "arrowright") &&
        Number.isFinite(duration) &&
        duration > 0
      ) {
        const currentTime =
          videoRef && typeof videoRef === "object" && "current" in videoRef
            ? (videoRef.current?.currentTime ?? playTime)
            : playTime;
        event.preventDefault();
        seekTo(
          Math.max(
            0,
            Math.min(duration, currentTime + (key === "arrowright" ? 5 : -5)),
          ),
        );
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [
    mode,
    status,
    playbackRate,
    setPlaybackRate,
    togglePlayPause,
    showFeedback,
    playTime,
    duration,
    videoRef,
    seekTo,
    selectedRendition,
    vodLevels,
    playingVODRendition,
  ]);

  return null;
}
