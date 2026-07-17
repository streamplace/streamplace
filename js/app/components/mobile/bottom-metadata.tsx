import {
  Avatar,
  ContentRights,
  ContentWarnings,
  formatActivity,
  formatHandle,
  formatHandleWithAt,
  IconButton,
  LiveBadge,
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
  useViews,
  zero,
} from "@streamplace/components";
import AQLink from "components/aqlink";
import FollowButton from "components/follow-button";
import { ChevronLeft, ChevronRight } from "lucide-react-native";
import { Linking, Pressable, ScrollView, View } from "react-native";
import type {
  ActivityGame,
  ActivityLabel,
} from "streamplace/src/lexicons/types/place/stream/defs";
import { LANG_TAG_PREFIX, LANGUAGES } from "../live-dashboard/livestream-panel";
import { KebabMenu } from "./desktop-ui/kebab";

const { gap, px, py, r } = zero;

const ATMOCO_STREAMS = [
  { handle: "stream1.atmosphereconf.org", label: "Great Hall" },
  { handle: "stream2.atmosphereconf.org", label: "Performance Theatre" },
  { handle: "stream3.atmosphereconf.org", label: "Room 2301" },
];

function AtMoCoNav({ currentHandle }: { currentHandle: string }) {
  const { theme } = useTheme();
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
      <Text size="sm" color="muted">
        Switch streams:
      </Text>
      {ATMOCO_STREAMS.map((stream) => {
        const isActive = currentHandle === stream.handle;
        return (
          <AQLink
            key={stream.handle}
            to={{ screen: "Stream", params: { user: stream.handle } }}
            style={[
              zero.px[3],
              r.full,
              {
                borderWidth: 1,
                borderColor: isActive
                  ? theme.colors.primary
                  : theme.colors.border,
                backgroundColor: isActive
                  ? theme.colors.surface2
                  : "transparent",
              },
            ]}
          >
            <Text size="sm">{stream.label}</Text>
          </AQLink>
        );
      })}
    </View>
  );
}

/**
 * The metadata block below the player — YouTube grammar: title on top,
 * streamer identity row (avatar with live ring, handle, follow) beneath,
 * viewer count and actions on the right.
 */
export function BottomMetadata({
  setShowChat,
  showChat,
  compact = false,
}: {
  setShowChat: (show: boolean) => void;
  showChat: boolean;
  /** Tighter paddings and no chat toggle — mobile below-player placement */
  compact?: boolean;
}) {
  const profile = useAuthor();
  const ls = useLivestreamStore((x) => x.livestream);
  const segment = useLivestreamStore((x) => x.segment);
  const mode = usePlayerStore((x) => x.mode);
  const did = useDID();
  const title = useTitle();

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
  const isLive = mode === "live";

  // YouTube leads with the channel's display name; the handle is secondary.
  const displayName = profile?.displayName?.trim();
  const handleStr = profile ? formatHandleWithAt(profile) : "@user";
  const streamerName = displayName || handleStr;
  const streamerSubtitle = [displayName ? handleStr : null, activityLabel]
    .filter(Boolean)
    .join("  ·  ");
  const views = useViews();

  return (
    <View
      style={[
        layout.position.relative,
        // Desktop: flush with the player's rounded edges, no divider box —
        // the metadata reads as part of the video composition (YouTube).
        // Compact/mobile keeps its own padding + hairline.
        compact
          ? {
              backgroundColor: theme.colors.surface0,
              borderTopWidth: 1,
              borderTopColor: theme.colors.borderSubtle,
            }
          : null,
        compact ? px[4] : px[0],
        compact ? py[2] : py[3],
      ]}
    >
      {/* LIVE status + watching count (YouTube-style live indicator) */}
      {isLive ? (
        <View style={[py[1]]}>
          <LiveBadge count={views ?? undefined} />
        </View>
      ) : null}

      {/* Title */}
      {title ? (
        <Text
          numberOfLines={2}
          ellipsizeMode="tail"
          size="lg"
          weight="semibold"
          style={[py[1]]}
        >
          {title}
        </Text>
      ) : null}

      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          gap.all[3],
        ]}
      >
        {/* Left: streamer identity row */}
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            gap.all[3],
            { flex: 1, minWidth: 0 },
          ]}
        >
          <Avatar
            src={avatarUri}
            name={profile ? formatHandle(profile) : undefined}
            size={compact ? "lg" : "xl"}
            live={isLive}
          />

          <View style={{ flexShrink: 1, minWidth: 0 }}>
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
              <Text numberOfLines={1} weight="medium">
                {streamerName}
              </Text>
            </Pressable>
            {streamerSubtitle ? (
              <Text numberOfLines={1} size="sm" color="muted">
                {streamerSubtitle}
              </Text>
            ) : null}
          </View>

          {did && profile && (
            <FollowButton streamerDID={profile.did} currentUserDID={did} />
          )}
        </View>

        {/* Right: viewers + actions */}
        <View style={[layout.flex.row, layout.flex.align.center, gap.all[2]]}>
          {!isLive ? <PlayerUI.Viewers /> : null}
          <ShareSheet />
          <KebabMenu />
          {isLive && !compact && (
            <IconButton
              variant="secondary"
              size="sm"
              accessibilityLabel={showChat ? "Hide chat" : "Show chat"}
              onPress={() => setShowChat(!showChat)}
            >
              {showChat ? (
                <ChevronRight color={theme.colors.text2} size={16} />
              ) : (
                <ChevronLeft color={theme.colors.text2} size={16} />
              )}
            </IconButton>
          )}
        </View>
      </View>

      {/* Tags */}
      {hasMeta && tags && tags.length > 0 && (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={[gap.all[2], layout.flex.alignCenter, py[1]]}
        >
          {tags.map((tag) => (
            <View
              key={tag}
              style={[
                r.full,
                px[2],
                {
                  borderWidth: 1,
                  borderColor: theme.colors.borderSubtle,
                  backgroundColor: theme.colors.surface2,
                },
              ]}
            >
              <Text size="xs" color="muted">
                {tag}
              </Text>
            </View>
          ))}
        </ScrollView>
      )}

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
