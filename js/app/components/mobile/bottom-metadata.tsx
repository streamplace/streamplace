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
  zero,
} from "@streamplace/components";
import FollowButton from "components/follow-button";
import { ChevronLeft, ChevronRight } from "lucide-react-native";
import { Image, Linking, Pressable, View } from "react-native";
const { gap, px, py, colors } = zero;

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
              resizeMode="cover"
            />
          )}
          {!(profile?.did && avatars[profile?.did]?.avatar) && (
            <Image
              key="avatar"
              source={require("./../../assets/images/goose.png")}
              style={{ width: 42, height: 42, borderRadius: 999 }}
              resizeMode="cover"
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
    </View>
  );
}
