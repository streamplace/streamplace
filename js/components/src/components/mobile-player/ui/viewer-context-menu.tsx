import { Cog } from "lucide-react-native";
import { useState } from "react";
import { Platform, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  withTiming,
} from "react-native-reanimated";
import { useLivestreamInfo, useStreamplaceStore, zero } from "../../..";
import { useLivestreamStore } from "../../../livestream-store";
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
  const setLowLatency = (value: boolean) => {
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
                  {qualities.map((r) => (
                    <DropdownMenuRadioItem key={r.name} value={r.name}>
                      <Text>{r.name === "audio" ? "Audio Only" : r.name}</Text>
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuGroup>
              {mode !== "vod" && (
                <>
                  <DropdownMenuGroup>
                    <DropdownMenuCheckboxItem
                      checked={lowLatency}
                      onCheckedChange={() => setLowLatency(!lowLatency)}
                    >
                      <Text>Low Latency</Text>
                    </DropdownMenuCheckboxItem>
                  </DropdownMenuGroup>
                  <DropdownMenuInfo description="Reduces the delay between video and chat for a more real-time experience." />
                </>
              )}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        </DropdownMenuGroup>
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
