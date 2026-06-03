import {
  Button,
  ContentRights,
  ContentWarnings,
  formatActivity,
  formatHandle,
  formatHandleWithAt,
  hexToRgba,
  layout,
  PlayerUI,
  ShareSheet,
  Text,
  useAuthor,
  useAvatar,
  useDID,
  useLivestreamStore,
  usePlayerStore,
  useTheme,
  useTitle,
  zero,
} from "@streamplace/components";
import AQLink from "components/aqlink";
import FollowButton from "components/follow-button";
import { Image } from "expo-image";
import { ChevronLeft, ChevronRight } from "lucide-react-native";
import { Linking, Pressable, ScrollView, View } from "react-native";
import type {
  ActivityGame,
  ActivityLabel,
} from "streamplace/src/lexicons/types/place/stream/defs";
import { LANG_TAG_PREFIX, LANGUAGES } from "../live-dashboard/livestream-panel";
import { KebabMenu } from "./desktop-ui/kebab";

const { gap, px, py, colors, r, borders } = zero;

const ROSE_RING = "rgba(244, 114, 182, 0.32)";

const ATMOCO_STREAMS = [
  { handle: "stream1.atmosphereconf.org", label: "Great Hall" },
  { handle: "stream2.atmosphereconf.org", label: "Performance Theatre" },
  { handle: "stream3.atmosphereconf.org", label: "Room 2301" },
];

function AtMoCoNav({ currentHandle }: { currentHandle: string }) {
  const z = useTheme();
  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[2],
        py[2],
        { flexWrap: "wrap" },
      ]}
    >
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
  const profile = useAuthor();
  const ls = useLivestreamStore((x) => x.livestream);
  const segment = useLivestreamStore((x) => x.segment);
  const mode = usePlayerStore((x) => x.mode);
  const did = useDID();

  const contentWarnings =
    (segment?.contentWarnings?.warnings as string[]) || [];
  const contentRights = segment?.contentRights;

  const { theme } = useTheme();
  const avatarUri = useAvatar();
  const activity = ls?.record.activity as
    | ((ActivityGame | ActivityLabel) & { $type?: string })
    | undefined;
  const activityLabel = activity ? formatActivity(activity) : null;
  const rawTags = ls?.record.tags as string[] | undefined;
  const tags = rawTags?.map((tag) => {
    if (tag.startsWith(LANG_TAG_PREFIX)) {
      const code = tag.slice(LANG_TAG_PREFIX.length);
      return LANGUAGES.find((l) => l.code === code)?.native ?? tag;
    }
    return tag;
  });
  const hasMeta = activityLabel || (tags && tags.length > 0);

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
          layout.flex.alignCenter,
          { flex: "auto" as any },
        ]}
      >
        {/* Left: avatar + text stack */}
        <View
          style={[
            layout.flex.row,
            layout.flex.center,
            gap.all[3],
            { flex: 1, minWidth: 0 },
          ]}
        >
          {/* Avatar with rose ring */}
          <View
            style={{
              width: 48,
              height: 48,
              borderRadius: 999,
              flexShrink: 0,
            }}
          >
            <Image
              key="avatar"
              source={
                avatarUri
                  ? { uri: avatarUri }
                  : require("./../../assets/images/goose.png")
              }
              style={{ width: "100%", height: "100%", borderRadius: 999 }}
              contentFit="cover"
            />
          </View>

          <View style={{ flex: 1, minWidth: 0 }}>
            {/* Handle + follow */}
            <View
              style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
            >
              <Pressable
                onPress={() => {
                  if (profile?.handle) {
                    Linking.openURL(
                      `https://bsky.app/profile/${formatHandle(profile)}`,
                    );
                  }
                }}
                style={{ flexShrink: 1, minWidth: 0 }}
              >
                <Text
                  numberOfLines={1}
                  color={hexToRgba(theme.colors.primaryForeground, 0.85)}
                >
                  {profile ? formatHandleWithAt(profile) : "@user"}
                </Text>
              </Pressable>
              {did && profile && (
                <FollowButton streamerDID={profile.did} currentUserDID={did} />
              )}
            </View>

            {/* Title */}
            <Text numberOfLines={3} ellipsizeMode="tail" weight="semibold">
              {useTitle()}
            </Text>

            {/* Activity + tags */}
            {hasMeta && (
              <ScrollView
                horizontal
                showsHorizontalScrollIndicator={false}
                contentContainerStyle={{ gap: 6, alignItems: "center" }}
              >
                {activityLabel && (
                  <Text style={{ color: theme.colors.ring }}>
                    {activityLabel}
                  </Text>
                )}
                {tags?.map((tag) => (
                  <View
                    key={tag}
                    style={[
                      r.full,
                      {
                        borderWidth: 1,
                        borderColor: theme.colors.border,
                        backgroundColor: hexToRgba(theme.colors.secondary, 0.3),
                        paddingHorizontal: 8,
                      },
                    ]}
                  >
                    <Text
                      size="sm"
                      color={hexToRgba(theme.colors.primaryForeground, 0.85)}
                    >
                      {tag}
                    </Text>
                  </View>
                ))}
              </ScrollView>
            )}
          </View>
        </View>

        {/* Right: controls */}
        <View style={[layout.flex.row, layout.flex.align.center, gap.all[6]]}>
          <PlayerUI.Viewers />
          <ShareSheet />
          <KebabMenu />
          {mode === "live" && (
            <View>
              <Button
                variant="outline"
                size="sm"
                width="min"
                style={{ aspectRatio: 1 }}
                onPress={() => setShowChat(!showChat)}
              >
                {showChat ? (
                  <ChevronRight color="white" size={16} />
                ) : (
                  <ChevronLeft color="white" size={16} />
                )}
              </Button>
            </View>
          )}
        </View>
      </View>

      {/* Content metadata */}
      {(contentWarnings.length > 0 ||
        (contentRights && Object.keys(contentRights).length > 0)) && (
        <View style={[py[2]]}>
          <ContentWarnings warnings={contentWarnings} compact={true} />
          {contentRights && (
            <ContentRights contentRights={contentRights} compact={true} />
          )}
        </View>
      )}

      {/* Atmosphere Conference inter-stream nav. TODO: remove after conf */}
      {profile?.handle &&
        ATMOCO_STREAMS.some((s) => s.handle === profile.handle) && (
          <AtMoCoNav currentHandle={profile.handle} />
        )}
    </View>
  );
}
