import { useRootContext } from "@rn-primitives/dropdown-menu";
import { Menu } from "lucide-react-native";
import { Image, Linking, Platform, Pressable, View } from "react-native";
import {
  ContentRights,
  ContentWarnings,
  formatHandle,
  formatHandleWithAt,
  useAvatars,
  useLivestreamInfo,
  zero,
} from "../../..";
import { colors } from "../../../lib/theme";
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
  DropdownMenuPortal,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
  Text,
} from "../../ui";

export function ContextMenu({
  dropdownPortalContainer,
}: {
  dropdownPortalContainer?: any;
}) {
  const quality = usePlayerStore((x) => x.selectedRendition);
  const setQuality = usePlayerStore((x) => x.setSelectedRendition);
  const qualities = useLivestreamStore((x) => x.renditions);

  const protocol = usePlayerStore((x) => x.protocol);
  const setProtocol = usePlayerStore((x) => x.setProtocol);

  const debugInfo = usePlayerStore((x) => x.showDebugInfo);
  const setShowDebugInfo = usePlayerStore((x) => x.setShowDebugInfo);

  const livestream = useLivestreamStore((x) => x.livestream);
  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const setReportSubject = usePlayerStore((x) => x.setReportSubject);

  const { profile } = useLivestreamInfo();

  const avatars = useAvatars(profile?.did ? [profile?.did] : []);
  const ls = useLivestreamStore((x) => x.livestream);
  const segment = useLivestreamStore((x) => x.segment);

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
  const Portal = isMobile ? View : DropdownMenuPortal;

  const DropdownMenuContent = ResponsiveDropdownMenuContent;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <Menu color={colors.gray[200]} />
      </DropdownMenuTrigger>
      <Portal container={dropdownPortalContainer}>
        <DropdownMenuContent side="top" align="end">
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
                {profile?.did && avatars[profile?.did]?.avatar && (
                  <Image
                    key="avatar"
                    source={{
                      uri: avatars[profile?.did]?.avatar,
                    }}
                    style={{ width: 42, height: 42, borderRadius: 999 }}
                    resizeMode="cover"
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
                    {ls?.record.title || "Stream Title"}
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
                    {quality === "source" ? "Source" : quality},{" "}
                    {lowLatency ? "Low Latency" : ""}
                  </Text>
                </View>
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <DropdownMenuGroup title="Resolution">
                  <DropdownMenuRadioGroup
                    value={quality}
                    onValueChange={setQuality}
                  >
                    <DropdownMenuRadioItem value="source">
                      <Text>Source (Original Quality)</Text>
                    </DropdownMenuRadioItem>
                    {qualities.map((r) => (
                      <DropdownMenuRadioItem key={r.name} value={r.name}>
                        <Text>{r.name}</Text>
                      </DropdownMenuRadioItem>
                    ))}
                  </DropdownMenuRadioGroup>
                </DropdownMenuGroup>
                <DropdownMenuGroup>
                  <DropdownMenuCheckboxItem
                    checked={lowLatency}
                    onCheckedChange={() => setLowLatency(!lowLatency)}
                  >
                    <Text>Low Latency</Text>
                  </DropdownMenuCheckboxItem>
                </DropdownMenuGroup>
                <DropdownMenuInfo description="Reduces the delay between video and chat for a more real-time experience." />
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
      </Portal>
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
