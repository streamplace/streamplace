import { Cog } from "lucide-react-native";
import { useState } from "react";
import { Platform, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  withTiming,
} from "react-native-reanimated";
import { useLivestreamInfo, useStreamplaceStore, zero } from "../../..";
import { useAudioStems, useLivestreamStore } from "../../../livestream-store";
import { PlayerProtocol, usePlayerStore } from "../../../player-store/";
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

// The stem volumes a viewer can pick; 0 silences the stem.
const STEM_LEVELS = [0, 0.25, 0.5, 1];
const stemLevelLabel = (level: number) =>
  level === 0 ? "Off" : `${level * 100}%`;

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

  // Only WebRTC carries the stems (HLS has just the full mix), so the Audio
  // section shows with low latency on.
  const stems = useAudioStems();
  const stemVolumes = usePlayerStore((x) => x.stemVolumes);
  const setStemVolumes = usePlayerStore((x) => x.setStemVolumes);

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

  const [isOpen, setIsOpen] = useState(false);

  const lowLatency = protocol === "webrtc";
  const streamForcesHLS = usePlayerStore((x) => x.streamForcesHLS);
  const setLowLatency = (value: boolean) => {
    if (streamForcesHLS) return;
    setProtocol(value ? PlayerProtocol.WEBRTC : PlayerProtocol.HLS);
  };

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
      <DropdownMenuTrigger testID="player-menu">
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
        {mode !== "vod" && lowLatency && stems.length > 0 && (
          <DropdownMenuGroup title="Audio">
            <DropdownMenuCheckboxItem
              testID="player-audio-full-mix"
              checked={stemVolumes === null}
              onCheckedChange={() =>
                setStemVolumes(stemVolumes === null ? {} : null)
              }
            >
              <Text>Full mix</Text>
            </DropdownMenuCheckboxItem>
            {stems.map((stem, i) => {
              const level = stemVolumes?.[stem.alternateGroup] ?? 1;
              return (
                <DropdownMenuSub key={stem.alternateGroup}>
                  <DropdownMenuSubTrigger
                    subMenuTitle={stem.label}
                    testID={`player-audio-stem-${i}`}
                  >
                    <View
                      style={[
                        zero.flex.values[1],
                        zero.layout.flex.row,
                        zero.layout.flex.spaceBetween,
                        zero.pr[4],
                      ]}
                    >
                      <Text>{stem.label}</Text>
                      {stemVolumes !== null && (
                        <Text muted size={isMobile ? "base" : "sm"}>
                          {stemLevelLabel(level)}
                        </Text>
                      )}
                    </View>
                  </DropdownMenuSubTrigger>
                  <DropdownMenuSubContent portalHost={dropdownPortalContainer}>
                    <DropdownMenuGroup title="Volume">
                      <DropdownMenuRadioGroup
                        value={stemVolumes === null ? "" : String(level)}
                        onValueChange={(value) =>
                          setStemVolumes({
                            ...stemVolumes,
                            [stem.alternateGroup]: Number(value),
                          })
                        }
                      >
                        {STEM_LEVELS.map((l) => (
                          <DropdownMenuRadioItem
                            key={l}
                            value={String(l)}
                            testID={`player-audio-stem-${i}-level-${l * 100}`}
                          >
                            <Text>{stemLevelLabel(l)}</Text>
                          </DropdownMenuRadioItem>
                        ))}
                      </DropdownMenuRadioGroup>
                    </DropdownMenuGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuSub>
              );
            })}
          </DropdownMenuGroup>
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
