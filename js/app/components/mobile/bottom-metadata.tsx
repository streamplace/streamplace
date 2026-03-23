import {
  Button,
  ContentRights,
  ContentWarnings,
  formatHandle,
  formatHandleWithAt,
  layout,
  PlayerUI,
  ShareSheet,
  Text,
  useAvatars,
  useDID,
  useLivestreamInfo,
  useLivestreamStore,
  useTheme,
  zero,
} from "@streamplace/components";
import AQLink from "components/aqlink";
import FollowButton from "components/follow-button";
import { Image } from "expo-image";
import { ChevronLeft, ChevronRight } from "lucide-react-native";
import { Linking, Pressable, View } from "react-native";
import { KebabMenu } from "./desktop-ui/kebab";
const { gap, px, py, colors } = zero;

const ATMOCO_STREAMS = [
  { handle: "stream1.atmosphereconf.org", label: "Great Hall" },
  { handle: "stream2.atmosphereconf.org", label: "Performance Theatre" },
  { handle: "stream3.atmosphereconf.org", label: "Room 2301" },
];

function AtMoCoNav({ currentHandle }: { currentHandle: string }) {
  const z = useTheme();
  return (
    <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2], py[2]]}>
      <Text>Switch streams:</Text>
      {ATMOCO_STREAMS.map((stream) => {
        const isActive = currentHandle === stream.handle;
        return (
          <AQLink
            key={stream.handle}
            to={{ screen: "Stream", params: { user: stream.handle } }}
            style={[
              zero.px[3],
              isActive
                ? { backgroundColor: z.theme.colors.accent }
                : zero.borders.width.thin,
              ,
              zero.borders.color.gray[500],
              zero.r.full,
            ]}
          >
            <Text>{stream.label}</Text>
          </AQLink>
        );
      })}
    </View>
  );
}

export function BottomMetadata({
  setShowChat,
  showChat,
}: {
  setShowChat: (show: boolean) => void;
  showChat: boolean;
}) {
  const { profile } = useLivestreamInfo();
  const avatars = useAvatars(profile?.did ? [profile?.did] : []);
  const ls = useLivestreamStore((x) => x.livestream);
  const segment = useLivestreamStore((x) => x.segment);

  const did = useDID();

  // Get content warnings and rights directly from the latest segment
  const contentWarnings =
    (segment?.contentWarnings?.warnings as string[]) || [];
  const contentRights = segment?.contentRights;

  return (
    <View
      style={[
        layout.position.relative,
        {
          backgroundColor: "rgba(0, 0, 0, 0.9)",
          borderTopWidth: 1,
          borderTopColor: "rgba(255, 255, 255, 0.1)",
        },
        px[5],
        py[3],
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          { height: "100%", flex: "auto" as any },
        ]}
      >
        {/* Left side - Profile info */}
        <View
          style={[
            layout.flex.row,
            layout.flex.center,
            gap.all[3],
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
              contentFit="cover"
            />
          )}
          {!(profile?.did && avatars[profile?.did]?.avatar) && (
            <Image
              key="avatar"
              source={require("./../../assets/images/goose.png")}
              style={{ width: 42, height: 42, borderRadius: 999 }}
              contentFit="cover"
            />
          )}
          <View style={{ flex: 1, minWidth: 0 }}>
            <View
              style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
            >
              <Pressable
                onPress={() => {
                  if (profile?.handle) {
                    const url = `https://bsky.app/profile/${formatHandle(profile)}`;
                    Linking.openURL(url);
                  }
                }}
              >
                <Text style={{ color: "white", fontWeight: "600" }}>
                  {profile ? formatHandleWithAt(profile) : "@user"}
                </Text>
              </Pressable>
              {did && profile && (
                <FollowButton streamerDID={profile?.did} currentUserDID={did} />
              )}
            </View>
            <Text
              style={{ color: colors.gray[400] }}
              numberOfLines={3}
              ellipsizeMode="tail"
            >
              {ls?.record.title || "Stream Title"}
            </Text>
          </View>
        </View>

        {/* Right side - Viewer count and collapse chat */}
        <View style={[layout.flex.row, layout.flex.align.center, gap.all[4]]}>
          <PlayerUI.Viewers />
          <ShareSheet />
          <KebabMenu />
          <View>
            <Button
              variant="outline"
              size="sm"
              width="min"
              style={{ aspectRatio: 1 }}
              onPress={() => {
                setShowChat(!showChat);
              }}
            >
              {showChat ? (
                <ChevronRight color="white" size={16} />
              ) : (
                <ChevronLeft color="white" size={16} />
              )}
            </Button>
          </View>
        </View>
      </View>

      {/* Content Metadata - Below the main profile/controls bar */}
      {(contentWarnings.length > 0 ||
        (contentRights && Object.keys(contentRights).length > 0)) && (
        <View style={[py[2]]}>
          <ContentWarnings warnings={contentWarnings} compact={true} />
          {contentRights && (
            <ContentRights contentRights={contentRights} compact={true} />
          )}
        </View>
      )}

      {/* Atmosphere Conference inter-stream navigation. TODO: remove after conf haha */}
      {profile?.handle &&
        ATMOCO_STREAMS.some((s) => s.handle === profile.handle) && (
          <AtMoCoNav currentHandle={profile.handle} />
        )}
    </View>
  );
}
