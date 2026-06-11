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
  /** Parent controls visibility — auto-hide logic lives in the HLSPlayer. */
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

  const visible = forceVisible || showControls || bigPlay;

  return (
    <div
      className={cn(
        "absolute inset-0 flex flex-col justify-end transition-opacity duration-200",
        visible ? "opacity-100" : "opacity-0 pointer-events-none",
      )}
    >
      {/* Top gradient — subtle hint that there's a controls bar.
          Not strictly needed since the bar has its own background, but
          gives the play button overlay a darker canvas. */}
      <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent pointer-events-none" />

      {bigPlay && (
        <button
          type="button"
          onClick={togglePlay}
          className="absolute inset-0 flex items-center justify-center pointer-events-auto"
          aria-label="Play"
        >
          <div className="flex items-center gap-3 bg-white/10 hover:bg-white/20 backdrop-blur px-5 py-3 rounded-full border border-white/20 transition-colors">
            <Play className="w-6 h-6 text-white fill-white" />
            <span className="text-white font-medium">Play</span>
          </div>
        </button>
      )}

      <div
        className="relative flex items-center gap-2 sm:gap-3 px-3 py-2 bg-gradient-to-t from-black/80 to-black/0 pointer-events-auto"
        // The wrapper itself is a "control surface" so clicks on the
        // gradient below the buttons don't bubble to the play handler.
        onClick={(e) => e.stopPropagation()}
      >
        <button
          type="button"
          onClick={togglePlay}
          className="text-white hover:text-white/80 transition-colors p-1"
          aria-label={playing ? "Pause" : "Play"}
        >
          {playing ? (
            <Pause className="w-5 h-5 fill-white" />
          ) : (
            <Play className="w-5 h-5 fill-white" />
          )}
        </button>

        {!isLive && (
          <span className="text-xs text-white/80 tabular-nums font-mono ml-1">
            {formatTime(currentTime)} / {formatTime(duration)}
          </span>
        )}

        <div className="flex items-center gap-1.5 group/vol">
          <button
            type="button"
            onClick={toggleMute}
            className="text-white hover:text-white/80 transition-colors p-1"
            aria-label={muted ? "Unmute" : "Mute"}
          >
            {muted || volume === 0 ? (
              <VolumeX className="w-5 h-5" />
            ) : (
              <Volume2 className="w-5 h-5" />
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
              "transition-colors p-1",
              isPiP ? "text-white" : "text-white/40 hover:text-white/80",
            )}
            aria-label={
              isPiP ? "Exit picture-in-picture" : "Picture-in-picture"
            }
          >
            <PictureInPicture className="w-5 h-5" />
          </button>
        )}

        <button
          type="button"
          onClick={() => setTheatre(!theatre)}
          className={cn(
            "transition-colors p-1",
            theatre ? "text-white" : "text-white/40 hover:text-white/80",
          )}
          aria-label={theatre ? "Exit theatre mode" : "Theatre mode"}
        >
          <RectangleHorizontal className="w-5 h-5" />
        </button>

        <button
          type="button"
          onClick={() => onShowDanmuChange(!showDanmu)}
          className={cn(
            "transition-colors p-1",
            showDanmu ? "text-white" : "text-white/40 hover:text-white/80",
          )}
          aria-label={showDanmu ? "Disable danmu" : "Enable danmu"}
        >
          <MuIcon size={20} />
        </button>

        {!isLive && duration > 0 && (
          <Slider
            min={0}
            max={duration}
            step={0.1}
            value={currentTime}
            onValueChange={onSeekInput}
            className="w-32 sm:w-48 accent-white h-1 cursor-pointer"
            aria-label="Seek"
          />
        )}

        <DropdownMenu onOpenChange={setSettingsOpen}>
          <DropdownMenuTrigger
            className="text-white hover:text-white/80 transition-colors p-1"
            aria-label="Settings"
          >
            <Settings
              className={cn(
                "w-5 h-5 transition-transform duration-300",
                settingsOpen && "rotate-60",
              )}
            />
          </DropdownMenuTrigger>
          <DropdownMenuContent
            side="top"
            align="end"
            className="bg-black/85 backdrop-blur border-white/10"
          >
            <DropdownMenuGroup>
              <DropdownMenuLabel>Latency</DropdownMenuLabel>
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
                  <DropdownMenuLabel>Quality</DropdownMenuLabel>
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
          className="text-white hover:text-white/80 transition-colors p-1"
          aria-label={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
        >
          {isFullscreen ? (
            <Minimize className="w-5 h-5" />
          ) : (
            <Maximize className="w-5 h-5" />
          )}
        </button>
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
