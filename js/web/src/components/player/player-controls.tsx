// Custom controls overlay for the HLS player. Replaces the native HTML5
// `controls` attribute so the player matches the rest of the web brand.
// Hides when the video is playing and the user is idle; stays visible
// when the video is paused or has errored.
import {
  Maximize,
  Minimize,
  Pause,
  PictureInPicture,
  Play,
  RectangleHorizontal,
  Settings,
  Volume2,
  VolumeX,
} from "lucide-react";
import { type RefObject, useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useFullscreen } from "../../contexts/fullscreen-context";
import { cn } from "../../lib/utils";
import MuIcon from "../svg/mu";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import { Slider } from "../ui/slider";
import type { QualityOption } from "./player";

export type PlayerControlsProps = {
  videoRef: RefObject<HTMLVideoElement | null>;
  /** Element to send into browser fullscreen. Defaults to the parent of the video. */
  containerRef: RefObject<HTMLElement | null>;
  /** Live streams hide the scrubber and show a "LIVE" badge in its place. */
  isLive: boolean;
  /** Parent controls visibility; auto-hide logic lives in the HLSPlayer. */
  showControls: boolean;
  /** When true, show a centered play button (no control bar). */
  bigPlay: boolean;
  /** When true, controls render at full opacity regardless of `showControls`. */
  forceVisible?: boolean;
  /** Available quality options from the backend. Empty hides the menu. */
  qualities: QualityOption[];
  /** Index of the currently selected quality (matches a `qualities[i].index`). */
  currentQuality: number;
  /** Request a quality change; the backend decides what to do. */
  onQualityChange: (index: number) => void;
  /** Whether the user picked low-latency (WebRTC) over standard (HLS). */
  useWebRTC: boolean;
  /** Toggle between standard (HLS) and low (WebRTC) transport. */
  onUseWebRTCChange: (useWebRTC: boolean) => void;
  /** Whether the "Stats for nerds" overlay is visible. */
  showStats: boolean;
  /** Toggle the stats overlay. */
  onShowStatsChange: (showStats: boolean) => void;
  /** Whether the danmu overlay is visible. */
  showDanmu: boolean;
  /** Toggle the danmu overlay. */
  onShowDanmuChange: (showDanmu: boolean) => void;
};

export function shouldShowUnmutePrompt(playing: boolean, muted: boolean) {
  return playing && muted;
}

export function PlayerControls({
  videoRef,
  containerRef,
  isLive,
  showControls,
  bigPlay,
  forceVisible,
  qualities,
  currentQuality,
  onQualityChange,
  useWebRTC,
  onUseWebRTCChange,
  showStats,
  onShowStatsChange,
  showDanmu,
  onShowDanmuChange,
}: PlayerControlsProps) {
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(true);
  const [volume, setVolume] = useState(1);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isPiP, setIsPiP] = useState(false);

  const [settingsOpen, setSettingsOpen] = useState(false);

  const { theatre, setTheatre } = useFullscreen();
  const { t } = useTranslation();

  // Mirror video element state into React.
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onVolumeChange = () => {
      setMuted(video.muted);
      setVolume(video.volume);
    };
    const onTimeUpdate = () => {
      if (!isLive) setCurrentTime(video.currentTime);
    };
    const onLoadedMetadata = () => {
      if (!isLive) setDuration(video.duration);
    };

    // Initialize from current state in case the video is already playing.
    setMuted(video.muted);
    setVolume(video.volume);
    if (!video.paused) setPlaying(true);

    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("volumechange", onVolumeChange);
    video.addEventListener("timeupdate", onTimeUpdate);
    video.addEventListener("loadedmetadata", onLoadedMetadata);

    return () => {
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("volumechange", onVolumeChange);
      video.removeEventListener("timeupdate", onTimeUpdate);
      video.removeEventListener("loadedmetadata", onLoadedMetadata);
    };
  }, [videoRef, isLive]);

  // Track browser fullscreen state.
  useEffect(() => {
    if (typeof document === "undefined") return;
    const onChange = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

  // Track PiP state.
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const onEnter = () => setIsPiP(true);
    const onLeave = () => setIsPiP(false);
    video.addEventListener("enterpictureinpicture", onEnter);
    video.addEventListener("leavepictureinpicture", onLeave);
    // Sync initial state.
    setIsPiP(!!document.pictureInPictureElement);
    return () => {
      video.removeEventListener("enterpictureinpicture", onEnter);
      video.removeEventListener("leavepictureinpicture", onLeave);
    };
  }, [videoRef]);

  const pipSupported =
    typeof document !== "undefined" && !!document.pictureInPictureEnabled;

  const togglePiP = useCallback(async () => {
    const video = videoRef.current;
    if (!video) return;
    try {
      if (document.pictureInPictureElement) {
        await document.exitPictureInPicture();
      } else {
        await video.requestPictureInPicture();
      }
    } catch {
      // PiP can be denied by the browser or user settings.
    }
  }, [videoRef]);

  const togglePlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      video.play().catch(() => {});
    } else {
      video.pause();
    }
  }, [videoRef]);

  const toggleMute = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    video.muted = !video.muted;
  }, [videoRef]);

  const onVolumeInput = useCallback(
    (v: number) => {
      const video = videoRef.current;
      if (!video) return;
      video.volume = v;
      // Unmuting requires volume > 0 on some browsers.
      if (v > 0) video.muted = false;
    },
    [videoRef],
  );

  const onSeekInput = useCallback(
    (currentTime: number) => {
      const video = videoRef.current;
      if (!video) return;
      video.currentTime = currentTime;
    },
    [videoRef],
  );

  const toggleFullscreen = useCallback(async () => {
    const el = containerRef.current;
    if (!el) return;
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await el.requestFullscreen();
      }
    } catch {
      // Fullscreen can be denied (e.g. not user-initiated in some browsers).
    }
  }, [containerRef]);

  // Keyboard shortcuts: space=play/pause, m=mute, f=fullscreen.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Don't hijack typing in inputs or anywhere a user is composing text
      // (contenteditable covers rich editors like the Tiptap chat input).
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") return;
      if (target?.isContentEditable) return;
      if (e.key === " ") {
        e.preventDefault();
        togglePlay();
      } else if (e.key === "m" || e.key === "M") {
        e.preventDefault();
        toggleMute();
      } else if (e.key === "f" || e.key === "F") {
        e.preventDefault();
        toggleFullscreen();
      } else if (e.key === "t" || e.key === "T") {
        e.preventDefault();
        setTheatre(!theatre);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [togglePlay, toggleMute, toggleFullscreen, theatre, setTheatre]);

  const showUnmutePrompt = shouldShowUnmutePrompt(playing, muted);
  const visible = forceVisible || showControls || bigPlay || showUnmutePrompt;

  return (
    <div
      className={cn(
        "absolute inset-0 flex flex-col justify-end transition-opacity duration-200",
        visible ? "opacity-100" : "pointer-events-none opacity-0",
      )}
    >
      {/* Top gradient; subtle hint that there's a controls bar.
          Not strictly needed since the bar has its own background, but
          gives the play button overlay a darker canvas. */}
      <div className="pointer-events-none absolute inset-0 bg-linear-to-t from-black/60 via-transparent to-transparent" />

      <button
        type="button"
        onClick={togglePlay}
        className={cn(
          "absolute inset-0 flex items-center justify-center transition-opacity duration-200 ease-out",
          bigPlay
            ? "pointer-events-auto opacity-100"
            : "pointer-events-none opacity-0",
        )}
        aria-label={t("player-play")}
        aria-hidden={!bigPlay}
        tabIndex={bigPlay ? 0 : -1}
      >
        <div className="flex items-center gap-3 rounded-full border border-white/20 bg-white/10 px-5 py-3 backdrop-blur transition-colors hover:bg-white/20">
          <Play className="h-6 w-6 fill-white text-white" />
          <span className="font-medium text-white">{t("player-play")}</span>
        </div>
      </button>

      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          toggleMute();
        }}
        className={cn(
          "focus-visible:ring-ring/50 absolute bottom-20 left-1/2 -translate-x-1/2 rounded-full transition-opacity duration-200 ease-out focus-visible:ring-3 focus-visible:outline-none",
          showUnmutePrompt
            ? "pointer-events-auto opacity-100"
            : "pointer-events-none opacity-0",
        )}
        aria-label={t("player-unmute")}
        aria-hidden={!showUnmutePrompt}
        tabIndex={showUnmutePrompt ? 0 : -1}
      >
        <div className="group border-destructive/40 bg-destructive/20 hover:bg-destructive/40 flex items-center gap-3 rounded-full border px-5 py-3 backdrop-blur transition-colors">
          <div>
            <VolumeX className="h-6 w-6 text-white" />
            <Volume2 className="text-success h-6 w-6" />
          </div>
          <span className="font-medium text-white">{t("player-unmute")}</span>
        </div>
      </button>

      <div
        className="pointer-events-auto relative space-y-2 bg-linear-to-t from-black/80 to-black/0 px-3 py-2 sm:gap-3"
        // The wrapper itself is a "control surface" so clicks on the
        // gradient below the buttons don't bubble to the play handler.
        onClick={(e) => e.stopPropagation()}
      >
        {!isLive && duration > 0 && (
          <Slider
            min={0}
            max={duration}
            step={0.1}
            value={currentTime}
            onValueChange={onSeekInput}
            className="h-1 w-full min-w-full cursor-pointer sm:w-48"
            aria-label={t("player-seek")}
          />
        )}
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={togglePlay}
            className="p-1 text-white transition-colors hover:text-white/80"
            aria-label={playing ? t("player-pause") : t("player-play")}
          >
            {playing ? (
              <Pause className="h-5 w-5 fill-white" />
            ) : (
              <Play className="h-5 w-5 fill-white" />
            )}
          </button>

          {!isLive && (
            <span className="flex font-mono text-sm text-white/80 tabular-nums">
              <div className="flex font-mono text-sm tabular-nums">
                {formatTime(currentTime)}
              </div>
              <div className="ml-1 flex font-mono text-sm tabular-nums">/</div>
              <div className="ml-1 flex font-mono text-sm tabular-nums">
                {formatTime(duration)}
              </div>
            </span>
          )}

          <div className="group/vol flex items-center gap-1.5">
            <button
              type="button"
              onClick={toggleMute}
              className="p-1 text-white transition-colors hover:text-white/80"
              aria-label={muted ? t("player-unmute") : t("player-mute")}
            >
              {muted || volume === 0 ? (
                <VolumeX className="h-5 w-5" />
              ) : (
                <Volume2 className="h-5 w-5" />
              )}
            </button>
            <Slider
              className="w-20 sm:w-24"
              min={0}
              max={1}
              step={0.01}
              value={muted ? 0 : volume}
              onValueChange={onVolumeInput}
            />
          </div>

          <div className="flex-1" />

          {pipSupported && (
            <button
              type="button"
              onClick={togglePiP}
              className={cn(
                "p-1 transition-colors",
                isPiP ? "text-white" : "text-white/40 hover:text-white/80",
              )}
              aria-label={isPiP ? t("player-exit-pip") : t("player-pip")}
            >
              <PictureInPicture className="h-5 w-5" />
            </button>
          )}

          <button
            type="button"
            onClick={() => setTheatre(!theatre)}
            className={cn(
              "p-1 transition-colors",
              theatre ? "text-white" : "text-white/40 hover:text-white/80",
            )}
            aria-label={
              theatre ? t("player-exit-theatre") : t("player-theatre")
            }
          >
            <RectangleHorizontal className="h-5 w-5" />
          </button>

          <button
            type="button"
            onClick={() => onShowDanmuChange(!showDanmu)}
            className={cn(
              "p-1 transition-colors",
              showDanmu ? "text-white" : "text-white/40 hover:text-white/80",
            )}
            aria-label={
              showDanmu ? t("player-disable-danmu") : t("player-enable-danmu")
            }
          >
            <MuIcon size={20} />
          </button>

          <DropdownMenu onOpenChange={setSettingsOpen}>
            <DropdownMenuTrigger
              className="p-1 text-white transition-colors hover:text-white/80"
              aria-label={t("player-settings")}
            >
              <Settings
                className={cn(
                  "h-5 w-5 transition-transform duration-300",
                  settingsOpen && "rotate-60",
                )}
              />
            </DropdownMenuTrigger>
            <DropdownMenuContent
              side="top"
              align="end"
              className="border-white/10 bg-black/85 backdrop-blur"
            >
              <DropdownMenuGroup>
                <DropdownMenuLabel>{t("player-latency")}</DropdownMenuLabel>
                <DropdownMenuRadioGroup
                  value={useWebRTC ? "webrtc" : "hls"}
                  onValueChange={(v) => onUseWebRTCChange(v === "webrtc")}
                >
                  <DropdownMenuRadioItem value="hls">
                    Standard
                  </DropdownMenuRadioItem>
                  <DropdownMenuRadioItem value="webrtc">
                    Low (WebRTC)
                  </DropdownMenuRadioItem>
                </DropdownMenuRadioGroup>
              </DropdownMenuGroup>

              {qualities.length > 0 && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuGroup>
                    <DropdownMenuLabel>{t("player-quality")}</DropdownMenuLabel>
                    <DropdownMenuRadioGroup
                      value={String(currentQuality)}
                      onValueChange={(v) => onQualityChange(Number(v))}
                    >
                      {qualities.map((q) => (
                        <DropdownMenuRadioItem
                          key={q.index}
                          value={String(q.index)}
                        >
                          {q.label}
                        </DropdownMenuRadioItem>
                      ))}
                    </DropdownMenuRadioGroup>
                  </DropdownMenuGroup>
                </>
              )}

              <DropdownMenuSeparator />
              <DropdownMenuCheckboxItem
                checked={showStats}
                onCheckedChange={onShowStatsChange}
              >
                Stats for nerds
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <button
            type="button"
            onClick={toggleFullscreen}
            className="p-1 text-white transition-colors hover:text-white/80"
            aria-label={
              isFullscreen
                ? t("player-exit-fullscreen")
                : t("player-fullscreen")
            }
          >
            {isFullscreen ? (
              <Minimize className="h-5 w-5" />
            ) : (
              <Maximize className="h-5 w-5" />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

function formatTime(s: number): string {
  if (!isFinite(s) || s < 0) return "0:00";
  const m = Math.floor(s / 60);
  const sec = Math.floor(s % 60);
  return `${m}:${sec.toString().padStart(2, "0")}`;
}
