import { useRootContext } from "@rn-primitives/dropdown-menu";
import { Image } from "expo-image";
import { Cog } from "lucide-react-native";
import { useState } from "react";
import { Linking, Platform, Pressable, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  withTiming,
} from "react-native-reanimated";
import {
  ContentRights,
  ContentWarnings,
  formatHandle,
  formatHandleWithAt,
  useAuthor,
  useAvatar,
  useStreamplaceStore,
  useTitle,
  zero,
} from "../../..";
import { useLivestreamStore } from "../../../livestream-store";
import { PlayerProtocol, usePlayerStore } from "../../../player-store/";
import { useGraphManager } from "../../../streamplace-store/graph";
import { gap, pt, px } from "../../../ui";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuInfo,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
  Text,
  useTheme,
} from "../../ui";

export function ContextMenu({
  dropdownPortalContainer,
}: {
  dropdownPortalContainer?: string;
}) {
  const th = useTheme();
  const quality = usePlayerStore((x) => x.selectedRendition);
  const setQuality = usePlayerStore((x) => x.setSelectedRendition);
  const mode = usePlayerStore((x) => x.mode);
  const vodLevels = usePlayerStore((x) => x.vodLevels);
  const playingVODRendition = usePlayerStore((x) => x.playingVODRendition);
  const liveRenditions = useLivestreamStore((x) => x.renditions);
  const qualities = mode === "vod" ? vodLevels : liveRenditions;

  const protocol = usePlayerStore((x) => x.protocol);
  const setProtocol = usePlayerStore((x) => x.setProtocol);

  const debugInfo = usePlayerStore((x) => x.showDebugInfo);
  const setShowDebugInfo = usePlayerStore((x) => x.setShowDebugInfo);

  const livestream = useLivestreamStore((x) => x.livestream);
  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const setReportSubject = usePlayerStore((x) => x.setReportSubject);

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

  const profile = useAuthor();

  const avatar = useAvatar();
  const segment = useLivestreamStore((x) => x.segment);

  const [isOpen, setIsOpen] = useState(false);

  // Get content rights from the latest segment
  const contentRights = segment?.contentRights;
  const contentWarnings = segment?.contentWarnings?.warnings || [];

  let graphManager = useGraphManager(profile?.did);

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
        {Platform.OS !== "web" && (
          <DropdownMenuGroup title="Streamer">
            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.center,
                zero.gap.all[3],
                { flex: 1, minWidth: 0 },
              ]}
            >
              {avatar && (
                <Image
                  key="avatar"
                  source={{
                    uri: avatar,
                  }}
                  style={{ width: 42, height: 42, borderRadius: 999 }}
                  contentFit="cover"
                />
              )}
              <View style={{ flex: 1, minWidth: 0 }}>
                <View
                  style={[
                    zero.layout.flex.row,
                    zero.layout.flex.alignCenter,
                    zero.gap.all[2],
                  ]}
                >
                  <Pressable
                    onPress={() => {
                      if (profile?.handle) {
                        const url = `https://bsky.app/profile/${formatHandle(profile)}`;
                        Linking.openURL(url);
                      }
                    }}
                  >
                    <Text>{profile && formatHandleWithAt(profile)}</Text>
                  </Pressable>
                  {/*{did && profile && (
                    <FollowButton streamerDID={profile?.did} currentUserDID={did} />
                  )}*/}
                </View>
                <Text
                  color="muted"
                  size="sm"
                  numberOfLines={2}
                  ellipsizeMode="tail"
                >
                  {useTitle()}
                </Text>
              </View>
            </View>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={graphManager.isLoading || !profile?.did}
              onPress={async () => {
                try {
                  if (graphManager.isFollowing) {
                    await graphManager.unfollow();
                  } else {
                    await graphManager.follow();
                  }
                } catch (err) {
                  console.error("Follow/unfollow error:", err);
                }
              }}
            >
              <Text
                color={graphManager.isFollowing ? "destructive" : "default"}
              >
                {graphManager.isLoading
                  ? "Loading..."
                  : graphManager.isFollowing
                    ? "Unfollow"
                    : "Follow"}
              </Text>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onPress={() => {
                if (profile?.handle) {
                  const url = `https://bsky.app/profile/${formatHandle(profile)}`;
                  Linking.openURL(url);
                }
              }}
            >
              <Text>View Profile on Bluesky</Text>
            </DropdownMenuItem>
          </DropdownMenuGroup>
        )}

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
        <DropdownMenuGroup title="Advanced">
          <DropdownMenuCheckboxItem
            checked={debugInfo}
            onCheckedChange={() => setShowDebugInfo(!debugInfo)}
          >
            <Text>Show Debug Info</Text>
          </DropdownMenuCheckboxItem>
        </DropdownMenuGroup>
        <DropdownMenuGroup title="Report">
          <ReportButton
            livestream={livestream}
            setReportModalOpen={setReportModalOpen}
            setReportSubject={setReportSubject}
          />
        </DropdownMenuGroup>
        <View style={[pt[3], px[2], gap.all[2]]}>
          {contentWarnings && contentWarnings.length > 0 && (
            <View style={[gap.all[1]]}>
              <Text size="base" color="muted">
                Stream may contain
              </Text>
              <ContentWarnings warnings={contentWarnings} compact={true} />
            </View>
          )}
          {contentRights && Object.keys(contentRights).length > 0 && (
            <ContentRights
              contentRights={contentRights}
              size="xs"
              color="muted"
            />
          )}
        </View>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ReportButton({
  livestream,
  setReportModalOpen,
  setReportSubject,
}) {
  const { onOpenChange } = useRootContext();
  return (
    <DropdownMenuItem
      onPress={() => {
        if (!livestream) return;
        onOpenChange?.(false);
        setReportModalOpen(true);
        setReportSubject({
          $type: "com.atproto.repo.strongRef",
          uri: livestream.uri,
          cid: livestream.cid,
        });
      }}
    >
      <Text>Report Livestream...</Text>
    </DropdownMenuItem>
  );
}
