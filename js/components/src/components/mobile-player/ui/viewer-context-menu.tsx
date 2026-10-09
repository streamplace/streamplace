import { Cog } from "lucide-react-native";
import { useCallback, useEffect, useState } from "react";
import { Platform, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  withTiming,
} from "react-native-reanimated";
import { useLivestreamInfo, useStreamplaceStore, zero } from "../../..";
import { useLivestreamStore } from "../../../livestream-store";
import {
  PlayerProtocol,
  PlayerStatus,
  usePlayerStore,
} from "../../../player-store/";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuInfo,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
  Text,
  useTheme,
} from "../../ui";
import { ReportMenuItems } from "./report-menu-items";

const PLAYBACK_RATES = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];

export function ContextMenu({
  dropdownPortalContainer,
  onOpenChat,
}: {
  dropdownPortalContainer?: string;
  onOpenChat?: () => void;
}) {
  const th = useTheme();
  const quality = usePlayerStore((x) => x.selectedRendition);
  const setQuality = usePlayerStore((x) => x.setSelectedRendition);
  const mode = usePlayerStore((x) => x.mode);
  const status = usePlayerStore((x) => x.status);
  const playbackRate = usePlayerStore((x) => x.playbackRate);
  const setPlaybackRate = usePlayerStore((x) => x.setPlaybackRate);
  const showFeedback = usePlayerStore((x) => x.showFeedback);
  const playTime = usePlayerStore((x) => x.playTime);
  const duration = usePlayerStore((x) => x.duration);
  const videoRef = usePlayerStore((x) => x.videoRef);
  const seekTo = usePlayerStore((x) => x.seekTo);
  const vodLevels = usePlayerStore((x) => x.vodLevels);
  const playingVODRendition = usePlayerStore((x) => x.playingVODRendition);
  const liveRenditions = useLivestreamStore((x) => x.renditions);
  const qualities = mode === "vod" ? vodLevels : liveRenditions;

  const livestream = useLivestreamStore((x) => x.livestream);
  const { profile } = useLivestreamInfo();
  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const setReportSubject = usePlayerStore((x) => x.setReportSubject);

  const protocol = usePlayerStore((x) => x.protocol);
  const setProtocol = usePlayerStore((x) => x.setProtocol);

  const isDevModeOn = useStreamplaceStore((x) => x.danmuUnlocked);

  const latestSegment = useLivestreamStore((x) => x.segment);
  // get highest height x width rendition for video
  const videoRendition = latestSegment?.video?.reduce((prev, current) => {
    const prevPixels = prev.width * prev.height;
    const currentPixels = current.width * current.height;
    return currentPixels > prevPixels ? current : prev;
  }, latestSegment?.video?.[0]);
  const highestLength = videoRendition
    ? videoRendition.height < videoRendition.width
      ? videoRendition.height
      : videoRendition?.width
    : 0;

  // ugh i hate this
  const frames = videoRendition?.framerate as
    | { num: number; den: number }
    | undefined;
  let fps =
    frames?.num && frames?.den
      ? Math.round((frames.num / frames.den) * 100) / 100
      : 0;

  if (!isDevModeOn && latestSegment?.video?.length) {
    fps = Math.round(fps);
  }

  const resolutionDisplay = highestLength
    ? `(${highestLength}p${fps > 0 ? fps : ""})`
    : "(Original Quality)";

  const frameRateName = quality === "source" ? playingVODRendition : quality;
  const vodFrameRate = vodLevels.find(
    (level) => level.name === frameRateName,
  )?.frameRate;
  const frameRate =
    vodFrameRate && Number.isFinite(vodFrameRate) && vodFrameRate > 0
      ? vodFrameRate
      : 30;
  const canStepFrames =
    quality !== "audio" &&
    status !== PlayerStatus.PLAYING &&
    Number.isFinite(duration) &&
    duration > 0;

  const [isOpen, setIsOpen] = useState(false);

  const lowLatency = protocol === "webrtc";
  const streamForcesHLS = usePlayerStore((x) => x.streamForcesHLS);
  const setLowLatency = (value: boolean) => {
    if (streamForcesHLS) return;
    setProtocol(value ? PlayerProtocol.WEBRTC : PlayerProtocol.HLS);
  };

  const changePlaybackRate = useCallback(
    (rate: number) => {
      if (mode !== "vod") return;
      const nextRate = Math.max(0.25, Math.min(2, rate));
      setPlaybackRate(nextRate);
      showFeedback({ type: "speed", playbackRate: nextRate });
    },
    [mode, setPlaybackRate, showFeedback],
  );

  const stepFrame = useCallback(
    (direction: "back" | "forward") => {
      if (mode !== "vod" || !canStepFrames) return;
      const ref = videoRef;
      const currentTime =
        ref && typeof ref === "object" && "current" in ref && ref.current
          ? ref.current.currentTime
          : playTime;
      const targetTime = Math.max(
        0,
        Math.min(
          duration,
          currentTime + (direction === "forward" ? 1 : -1) / frameRate,
        ),
      );
      seekTo(targetTime);
      showFeedback({ type: "frame", direction });
    },
    [
      mode,
      canStepFrames,
      videoRef,
      playTime,
      duration,
      frameRate,
      seekTo,
      showFeedback,
    ],
  );

  useEffect(() => {
    if (
      Platform.OS !== "web" ||
      mode !== "vod" ||
      typeof window === "undefined"
    ) {
      return;
    }
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
        target?.closest?.(
          'input, textarea, select, button, a, [contenteditable], [role="textbox"], [role="slider"], [role="menu"], [role="dialog"]',
        ) ||
        isOpen
      ) {
        return;
      }

      const key = event.key.toLowerCase();
      if (key === "<" || key === ">") {
        event.preventDefault();
        changePlaybackRate(playbackRate + (key === ">" ? 0.25 : -0.25));
      } else if ((key === "," || key === ".") && canStepFrames) {
        event.preventDefault();
        stepFrame(key === "." ? "forward" : "back");
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [
    mode,
    isOpen,
    playbackRate,
    canStepFrames,
    changePlaybackRate,
    stepFrame,
  ]);

  // are we on mobile? then do dropdowns
  const isMobile = Platform.OS === "ios" || Platform.OS === "android";

  // dummy portal for mobile
  //const Portal: typeof DropdownMenuPortal = DropdownMenu;

  const DropdownMenuContent = ResponsiveDropdownMenuContent;

  const iconRotate = useAnimatedStyle(() => {
    return {
      transform: [
        {
          rotateZ: withTiming(isOpen ? "240deg" : "0deg", {
            duration: 650,
            easing: Easing.out(Easing.ease),
          }),
        },
      ],
    };
  });

  // rerender when dropdown portal container changes so we swap portals 'seamlessly'
  return (
    <DropdownMenu onOpenChange={setIsOpen} key={dropdownPortalContainer}>
      <DropdownMenuTrigger>
        <Animated.View style={[iconRotate]}>
          <Cog color={th.theme.colors.foreground} />
        </Animated.View>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="end"
        portalHost={dropdownPortalContainer}
      >
        <DropdownMenuGroup>
          <DropdownMenuSub>
            <DropdownMenuSubTrigger subMenuTitle="Quality">
              <View
                style={[
                  zero.flex.values[1],
                  isMobile ? zero.layout.flex.row : zero.layout.flex.column,
                  zero.layout.flex.spaceBetween,
                  zero.pr[4],
                ]}
              >
                <Text>Quality</Text>
                <Text muted size={isMobile ? "base" : "sm"}>
                  {quality === "source"
                    ? mode === "vod"
                      ? `Auto${playingVODRendition ? ` (${playingVODRendition})` : ""}\n`
                      : `Source${resolutionDisplay ? " " + resolutionDisplay + "\n" : ", "}`
                    : quality === "audio"
                      ? `Audio Only\n`
                      : quality}
                  {mode !== "vod" && lowLatency ? "Low Latency" : ""}
                </Text>
              </View>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent portalHost={dropdownPortalContainer}>
              <DropdownMenuGroup title="Resolution">
                <DropdownMenuRadioGroup
                  value={quality}
                  onValueChange={setQuality}
                >
                  <DropdownMenuRadioItem value="source">
                    <Text>Source {resolutionDisplay}</Text>
                  </DropdownMenuRadioItem>
                  {qualities
                    .filter((r) => r.name !== "audio")
                    .map((r) => (
                      <DropdownMenuRadioItem key={r.name} value={r.name}>
                        <Text>{r.name}</Text>
                      </DropdownMenuRadioItem>
                    ))}
                </DropdownMenuRadioGroup>
                {qualities.some((r) => r.name === "audio") && (
                  // A checkbox rather than a radio entry: choosing it again
                  // turns audio-only back off and returns to the source.
                  <DropdownMenuCheckboxItem
                    checked={quality === "audio"}
                    onCheckedChange={() =>
                      setQuality(quality === "audio" ? "source" : "audio")
                    }
                  >
                    <Text>Audio Only</Text>
                  </DropdownMenuCheckboxItem>
                )}
              </DropdownMenuGroup>
              {mode !== "vod" && (
                <>
                  <DropdownMenuGroup>
                    <DropdownMenuCheckboxItem
                      checked={lowLatency}
                      disabled={streamForcesHLS}
                      onCheckedChange={() => setLowLatency(!lowLatency)}
                    >
                      <Text muted={streamForcesHLS}>Low Latency</Text>
                    </DropdownMenuCheckboxItem>
                  </DropdownMenuGroup>
                  <DropdownMenuInfo
                    description={
                      streamForcesHLS
                        ? "Not available for this stream: its video has B-frames, which low-latency playback can't handle."
                        : "Reduces the delay between video and chat for a more real-time experience."
                    }
                  />
                </>
              )}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        </DropdownMenuGroup>
        {mode === "vod" && (
          <>
            <DropdownMenuGroup>
              <DropdownMenuSub>
                <DropdownMenuSubTrigger subMenuTitle="Speed">
                  <View
                    style={[
                      zero.flex.values[1],
                      isMobile ? zero.layout.flex.row : zero.layout.flex.column,
                      zero.layout.flex.spaceBetween,
                      zero.pr[4],
                    ]}
                  >
                    <Text>Speed</Text>
                    <Text muted size={isMobile ? "base" : "sm"}>
                      {playbackRate}×
                    </Text>
                  </View>
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent portalHost={dropdownPortalContainer}>
                  <DropdownMenuGroup title="Playback speed">
                    <DropdownMenuRadioGroup
                      value={String(playbackRate)}
                      onValueChange={(value) =>
                        changePlaybackRate(Number(value))
                      }
                    >
                      {PLAYBACK_RATES.map((rate) => (
                        <DropdownMenuRadioItem key={rate} value={String(rate)}>
                          <Text>{rate === 1 ? "Normal" : `${rate}×`}</Text>
                        </DropdownMenuRadioItem>
                      ))}
                    </DropdownMenuRadioGroup>
                  </DropdownMenuGroup>
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </DropdownMenuGroup>
            <DropdownMenuGroup title="Frame">
              <DropdownMenuItem
                disabled={!canStepFrames}
                onPress={() => stepFrame("back")}
              >
                <Text>Previous frame</Text>
              </DropdownMenuItem>
              <DropdownMenuItem
                disabled={!canStepFrames}
                onPress={() => stepFrame("forward")}
              >
                <Text>Next frame</Text>
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </>
        )}
        {onOpenChat && (
          <DropdownMenuGroup title="View">
            <DropdownMenuItem closeOnPress={true} onPress={onOpenChat}>
              <Text>Chat-only mode</Text>
            </DropdownMenuItem>
          </DropdownMenuGroup>
        )}
        <ReportMenuItems
          livestream={livestream}
          profile={profile}
          setReportModalOpen={setReportModalOpen}
          setReportSubject={setReportSubject}
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
