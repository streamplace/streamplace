// Custom controls overlay for the HLS player. Replaces the native HTML5
// `controls` attribute so the player matches the rest of the web brand.
// Hides when the video is playing and the user is idle; stays visible
// when the video is paused or has errored.
import {
  FastForward,
  Gauge,
  type LucideIcon,
  Maximize,
  Minimize,
  Pause,
  PictureInPicture,
  Play,
  RectangleHorizontal,
  Rewind,
  Settings,
  StepBack,
  StepForward,
  Volume2,
  VolumeX,
} from "lucide-react";
import {
  type RefObject,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { useFullscreen } from "../../contexts/fullscreen-context";
import { cn } from "../../lib/utils";
import MuIcon from "../svg/mu";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import { Slider } from "../ui/slider";
import type { QualityOption } from "./player";

const PLAYBACK_RATES = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];
const FEEDBACK_DURATION_MS = 1500;

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
  const [playbackRate, setPlaybackRate] = useState(1);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isPiP, setIsPiP] = useState(false);
  const [feedback, setFeedback] = useState<{
    message: string;
    icon: LucideIcon;
  } | null>(null);
  const feedbackTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [settingsOpen, setSettingsOpen] = useState(false);

  const { theatre, setTheatre } = useFullscreen();
  const { t } = useTranslation();

  const showFeedback = useCallback((message: string, icon: LucideIcon) => {
    if (feedbackTimeout.current !== null) clearTimeout(feedbackTimeout.current);
    setFeedback({ message, icon });
    feedbackTimeout.current = setTimeout(() => {
      setFeedback(null);
      feedbackTimeout.current = null;
    }, FEEDBACK_DURATION_MS);
  }, []);

  useEffect(
    () => () => {
      if (feedbackTimeout.current !== null)
        clearTimeout(feedbackTimeout.current);
    },
    [],
  );

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
    const onRateChange = () => setPlaybackRate(video.playbackRate);

    // Sync initial state — the video may already be playing when we attach.
    setMuted(video.muted);
    setVolume(video.volume);
    if (!video.paused) setPlaying(true);
    onLoadedMetadata();
    onTimeUpdate();
    onRateChange();

    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("volumechange", onVolumeChange);
    video.addEventListener("timeupdate", onTimeUpdate);
    video.addEventListener("loadedmetadata", onLoadedMetadata);
    video.addEventListener("durationchange", onLoadedMetadata);
    video.addEventListener("ratechange", onRateChange);

    return () => {
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("volumechange", onVolumeChange);
      video.removeEventListener("timeupdate", onTimeUpdate);
      video.removeEventListener("loadedmetadata", onLoadedMetadata);
      video.removeEventListener("durationchange", onLoadedMetadata);
      video.removeEventListener("ratechange", onRateChange);
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
      showFeedback(
        t(document.pictureInPictureElement ? "player-exit-pip" : "player-pip"),
        PictureInPicture,
      );
      if (document.pictureInPictureElement) {
        await document.exitPictureInPicture();
      } else {
        await video.requestPictureInPicture();
      }
    } catch {
      // PiP can be denied by the browser or user settings.
    }
  }, [videoRef, showFeedback, t]);

  const togglePlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    showFeedback(
      t(video.paused ? "player-play" : "player-pause"),
      video.paused ? Play : Pause,
    );
    if (video.paused) {
      video.play().catch(() => {});
    } else {
      video.pause();
    }
  }, [videoRef, showFeedback, t]);

  const toggleMute = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    video.muted = !video.muted;
    showFeedback(
      t(video.muted ? "player-feedback-muted" : "player-feedback-unmuted"),
      video.muted ? VolumeX : Volume2,
    );
  }, [videoRef, showFeedback, t]);

  const onVolumeInput = useCallback(
    (v: number) => {
      const video = videoRef.current;
      if (!video) return;
      video.volume = v;
      // Unmuting requires volume > 0 on some browsers.
      if (v > 0) video.muted = false;
      showFeedback(
        t("player-feedback-volume", { volume: Math.round(video.volume * 100) }),
        video.volume > 0 ? Volume2 : VolumeX,
      );
    },
    [videoRef, showFeedback, t],
  );

  const onSeekInput = useCallback(
    (currentTime: number) => {
      const video = videoRef.current;
      if (!video) return;
      const icon = currentTime < video.currentTime ? Rewind : FastForward;
      video.currentTime = currentTime;
      setCurrentTime(video.currentTime);
      showFeedback(
        t("player-feedback-seek", { time: formatTime(video.currentTime) }),
        icon,
      );
    },
    [videoRef, showFeedback, t],
  );

  const onPlaybackRateChange = useCallback(
    (rate: number) => {
      const video = videoRef.current;
      if (!video || isLive) return;
      video.playbackRate = Math.max(0.25, Math.min(2, rate));
      setPlaybackRate(video.playbackRate);
      showFeedback(
        t("player-feedback-speed", { speed: video.playbackRate }),
        Gauge,
      );
    },
    [videoRef, isLive, showFeedback, t],
  );

  const frameRate = qualities.find(
    (q) => q.index === currentQuality,
  )?.frameRate;
  const stepFrame = useCallback(
    (direction: number) => {
      const video = videoRef.current;
      if (
        !video ||
        isLive ||
        !video.paused ||
        !Number.isFinite(video.duration) ||
        video.duration <= 0
      )
        return;
      // Native HLS and manifests without FRAME-RATE use a 30 fps estimate.
      const fps =
        frameRate && Number.isFinite(frameRate) && frameRate > 0
          ? frameRate
          : 30;
      video.currentTime = Math.max(
        0,
        Math.min(video.duration, video.currentTime + direction / fps),
      );
      setCurrentTime(video.currentTime);
      const milliseconds = Math.round(video.currentTime * 1000);
      showFeedback(
        t("player-feedback-frame", {
          frame: t(
            direction > 0 ? "player-frame-next" : "player-frame-previous",
          ),
          time: `${formatTime(milliseconds / 1000)}.${String(milliseconds % 1000).padStart(3, "0")}`,
        }),
        direction > 0 ? StepForward : StepBack,
      );
    },
    [videoRef, isLive, frameRate, showFeedback, t],
  );

  const toggleFullscreen = useCallback(async () => {
    const el = containerRef.current;
    if (!el) return;
    try {
      showFeedback(
        t(
          document.fullscreenElement
            ? "player-exit-fullscreen"
            : "player-fullscreen",
        ),
        document.fullscreenElement ? Minimize : Maximize,
      );
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await el.requestFullscreen();
      }
    } catch {
      // Fullscreen can be denied (e.g. not user-initiated in some browsers).
    }
  }, [containerRef, showFeedback, t]);

  const onTheatreChange = useCallback(
    (value: boolean) => {
      setTheatre(value);
      showFeedback(
        t(value ? "player-theatre" : "player-exit-theatre"),
        RectangleHorizontal,
      );
    },
    [setTheatre, showFeedback, t],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (
        e.defaultPrevented ||
        e.ctrlKey ||
        e.metaKey ||
        e.altKey ||
        e.isComposing
      )
        return;
      // Preserve typing, focused controls, and popup keyboard navigation.
      const target = e.target as HTMLElement | null;
      if (target?.isContentEditable) return;
      if (
        target?.closest?.(
          'input, textarea, select, button, a, [contenteditable], [role="textbox"], [role="slider"], [role="menu"], [role="dialog"]',
        )
      )
        return;
      if (settingsOpen) return;
      const video = videoRef.current;
      if (!video) return;
      const key = e.key.toLowerCase();
      if (key === " " || key === "k") {
        e.preventDefault();
        togglePlay();
      } else if (key === "m") {
        e.preventDefault();
        toggleMute();
      } else if (key === "f") {
        e.preventDefault();
        toggleFullscreen();
      } else if (key === "t") {
        e.preventDefault();
        onTheatreChange(!theatre);
      } else if (key === "arrowup" || key === "arrowdown") {
        e.preventDefault();
        onVolumeInput(
          Math.max(
            0,
            Math.min(1, video.volume + (key === "arrowup" ? 0.05 : -0.05)),
          ),
        );
      } else if (!isLive) {
        if (key === "<" || key === ">") {
          e.preventDefault();
          onPlaybackRateChange(
            video.playbackRate + (key === ">" ? 0.25 : -0.25),
          );
        } else if ((key === "," || key === ".") && video.paused) {
          e.preventDefault();
          stepFrame(key === "." ? 1 : -1);
        } else if (Number.isFinite(video.duration) && video.duration > 0) {
          let time: number;
          switch (key) {
            case "arrowleft":
              time = video.currentTime - 5;
              break;
            case "arrowright":
              time = video.currentTime + 5;
              break;
            case "j":
              time = video.currentTime - 10;
              break;
            case "l":
              time = video.currentTime + 10;
              break;
            case "home":
              time = 0;
              break;
            case "end":
              time = video.duration;
              break;
            default:
              if (!/^[0-9]$/.test(key)) return;
              time = (Number(key) / 10) * video.duration;
          }
          e.preventDefault();
          onSeekInput(Math.max(0, Math.min(video.duration, time)));
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [
    videoRef,
    togglePlay,
    toggleMute,
    toggleFullscreen,
    theatre,
    onTheatreChange,
    isLive,
    settingsOpen,
    onVolumeInput,
    onSeekInput,
    onPlaybackRateChange,
    stepFrame,
  ]);

  const showUnmutePrompt = shouldShowUnmutePrompt(playing, muted);
  const visible =
    forceVisible || showControls || bigPlay || showUnmutePrompt || settingsOpen;

  return (
    <div
      className={cn(
        "absolute inset-0 flex flex-col justify-end transition-opacity duration-200",
        visible ? "opacity-100" : "pointer-events-none opacity-0",
      )}
    >
      {feedback &&
        containerRef.current &&
        createPortal(
          <div
            role="status"
            aria-live="polite"
            aria-atomic="true"
            className="pointer-events-none absolute top-1/2 left-1/2 z-20 flex w-full -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-2 px-4 text-center text-sm font-medium text-white tabular-nums opacity-50"
          >
            <feedback.icon className="size-16" aria-hidden="true" />
            <span>{feedback.message}</span>
          </div>,
          containerRef.current,
        )}
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
        {!feedback && (
          <div className="flex items-center gap-3 rounded-full border border-white/20 bg-white/10 px-5 py-3 backdrop-blur transition-colors hover:bg-white/20">
            <Play className="h-6 w-6 fill-white text-white" />
            <span className="font-medium text-white">{t("player-play")}</span>
          </div>
        )}
      </button>

      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          toggleMute();
        }}
        className={cn(
          "focus-visible:ring-ring/50 wide:bottom-20 absolute bottom-16 left-1/2 -translate-x-1/2 rounded-full transition-opacity duration-200 ease-out focus-visible:ring-3 focus-visible:outline-none",
          showUnmutePrompt
            ? "pointer-events-auto opacity-100"
            : "pointer-events-none opacity-0",
        )}
        aria-label={t("player-unmute")}
        aria-hidden={!showUnmutePrompt}
        tabIndex={showUnmutePrompt ? 0 : -1}
      >
        <div className="group bg-primary/50 hover:bg-primary/80 flex h-11 items-center gap-2 rounded-full border border-white/20 px-4 backdrop-blur transition-colors">
          <VolumeX className="text-primary-foreground h-5 w-5" />
          <span className="text-primary-foreground font-medium">
            {t("player-unmute")}
          </span>
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
        <div className="wide:gap-2 flex items-center gap-0.5">
          <button
            type="button"
            onClick={togglePlay}
            className="wide:size-auto wide:p-1 flex size-11 items-center justify-center rounded-md text-white transition-colors hover:bg-white/10 hover:text-white/80"
            aria-label={playing ? t("player-pause") : t("player-play")}
            title={playing ? t("player-pause") : t("player-play")}
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
              className="wide:size-auto wide:p-1 flex size-11 items-center justify-center rounded-md text-white transition-colors hover:bg-white/10 hover:text-white/80"
              aria-label={muted ? t("player-unmute") : t("player-mute")}
              title={muted ? t("player-unmute") : t("player-mute")}
            >
              {muted || volume === 0 ? (
                <VolumeX className="h-5 w-5" />
              ) : (
                <Volume2 className="h-5 w-5" />
              )}
            </button>
            <Slider
              className="wide:flex wide:w-24 hidden w-20"
              min={0}
              max={1}
              step={0.01}
              value={muted ? 0 : volume}
              onValueChange={onVolumeInput}
              aria-label={t("player-volume", { defaultValue: "Volume" })}
            />
          </div>

          <div className="flex-1" />

          {pipSupported && (
            <button
              type="button"
              onClick={togglePiP}
              className={cn(
                "wide:size-auto wide:p-1 flex size-11 items-center justify-center rounded-md transition-colors hover:bg-white/10",
                isPiP ? "text-white" : "text-white/40 hover:text-white/80",
              )}
              aria-label={isPiP ? t("player-exit-pip") : t("player-pip")}
              title={isPiP ? t("player-exit-pip") : t("player-pip")}
            >
              <PictureInPicture className="h-5 w-5" />
            </button>
          )}

          <button
            type="button"
            onClick={() => onTheatreChange(!theatre)}
            className={cn(
              "wide:flex wide:p-1 hidden transition-colors",
              theatre ? "text-white" : "text-white/40 hover:text-white/80",
            )}
            aria-label={
              theatre ? t("player-exit-theatre") : t("player-theatre")
            }
            title={theatre ? t("player-exit-theatre") : t("player-theatre")}
          >
            <RectangleHorizontal className="h-5 w-5" />
          </button>

          <button
            type="button"
            onClick={() => onShowDanmuChange(!showDanmu)}
            className={cn(
              "wide:flex wide:p-1 hidden transition-colors",
              showDanmu ? "text-white" : "text-white/40 hover:text-white/80",
            )}
            aria-label={
              showDanmu ? t("player-disable-danmu") : t("player-enable-danmu")
            }
            title={
              showDanmu ? t("player-disable-danmu") : t("player-enable-danmu")
            }
          >
            <MuIcon size={20} />
          </button>

          <DropdownMenu onOpenChange={setSettingsOpen}>
            <DropdownMenuTrigger
              className="wide:size-auto wide:p-1 flex size-11 items-center justify-center rounded-md text-white transition-colors hover:bg-white/10 hover:text-white/80"
              aria-label={t("player-settings")}
              title={t("player-settings")}
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
              portalContainer={containerRef.current}
              className="border-border bg-popover/95 min-w-48 backdrop-blur"
            >
              {!isLive && (
                <>
                  <DropdownMenuSub>
                    <DropdownMenuSubTrigger>
                      {t("player-speed")}
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent
                      portalContainer={containerRef.current}
                    >
                      <DropdownMenuGroup>
                        <DropdownMenuRadioGroup
                          value={String(playbackRate)}
                          onValueChange={(v) => onPlaybackRateChange(Number(v))}
                        >
                          {PLAYBACK_RATES.map((rate) => (
                            <DropdownMenuRadioItem
                              key={rate}
                              value={String(rate)}
                            >
                              {rate === 1
                                ? t("player-speed-normal")
                                : `${rate}×`}
                            </DropdownMenuRadioItem>
                          ))}
                        </DropdownMenuRadioGroup>
                      </DropdownMenuGroup>
                    </DropdownMenuSubContent>
                  </DropdownMenuSub>
                  <DropdownMenuSub>
                    <DropdownMenuSubTrigger>
                      {t("player-frame")}
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent
                      portalContainer={containerRef.current}
                    >
                      <DropdownMenuGroup>
                        <DropdownMenuLabel>
                          {t("player-frame-paused")}
                        </DropdownMenuLabel>
                        <DropdownMenuItem
                          disabled={
                            playing ||
                            !Number.isFinite(duration) ||
                            duration <= 0
                          }
                          onClick={() => stepFrame(-1)}
                        >
                          {t("player-frame-previous")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          disabled={
                            playing ||
                            !Number.isFinite(duration) ||
                            duration <= 0
                          }
                          onClick={() => stepFrame(1)}
                        >
                          {t("player-frame-next")}
                        </DropdownMenuItem>
                      </DropdownMenuGroup>
                    </DropdownMenuSubContent>
                  </DropdownMenuSub>
                  <DropdownMenuSeparator />
                </>
              )}
              <DropdownMenuGroup>
                <DropdownMenuLabel>{t("player-latency")}</DropdownMenuLabel>
                <DropdownMenuRadioGroup
                  value={useWebRTC ? "webrtc" : "hls"}
                  onValueChange={(v) => onUseWebRTCChange(v === "webrtc")}
                >
                  <DropdownMenuRadioItem value="hls">
                    {t("player-latency-standard")}
                  </DropdownMenuRadioItem>
                  <DropdownMenuRadioItem value="webrtc">
                    {t("player-latency-low")}
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
                className="wide:hidden"
                checked={theatre}
                onCheckedChange={onTheatreChange}
              >
                {t("player-theatre")}
              </DropdownMenuCheckboxItem>
              <DropdownMenuCheckboxItem
                className="wide:hidden"
                checked={showDanmu}
                onCheckedChange={onShowDanmuChange}
              >
                {t("player-danmu")}
              </DropdownMenuCheckboxItem>
              <DropdownMenuCheckboxItem
                checked={showStats}
                onCheckedChange={onShowStatsChange}
              >
                {t("player-stats")}
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <button
            type="button"
            onClick={toggleFullscreen}
            className="wide:size-auto wide:p-1 flex size-11 items-center justify-center rounded-md text-white transition-colors hover:bg-white/10 hover:text-white/80"
            aria-label={
              isFullscreen
                ? t("player-exit-fullscreen")
                : t("player-fullscreen")
            }
            title={
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
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = Math.floor(s % 60);
  if (h === 0) return `${m}:${sec.toString().padStart(2, "0")}`;
  return `${h}:${m.toString().padStart(2, "0")}:${sec.toString().padStart(2, "0")}`;
}
