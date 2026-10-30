import {
  Avatar,
  ContentRights,
  ContentWarnings,
  formatActivity,
  IconButton,
  layout,
  ShareSheet,
  Text,
  useAuthor,
  useAvatar,
  useDID,
  useLivestreamStore,
  usePlayerStore,
  useTheme,
  useTitle,
  useTranslation,
  useViews,
  zero,
} from "@streamplace/components";
import {
  motion,
  spacing,
  statusColors,
  tabularNums,
} from "@streamplace/components/src/lib/theme/tokens";
import { formatHandle, formatHandleWithAt } from "@streamplace/core";
import AQLink from "components/aqlink";
import FollowButton from "components/follow-button";
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
} from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import { Linking, Pressable, ScrollView, View } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import type { place } from "streamplace";
import { LANG_TAG_PREFIX, LANGUAGES } from "../live-dashboard/livestream-panel";
import { KebabMenu } from "./desktop-ui/kebab";
const { gap, pb, pt, px, py, r, borders } = zero;

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
 * Stream identity and wrapping tags below the player. Rights stay in Details
 * so they remain accessible without competing with the title and creator.
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
  const {
    profile,
    did,
    title,
    avatarUri,
    isLive: isLiveMode,
    livestream,
    segment,
    views,
    tags,
    contentRights,
    handleStr,
    streamerName,
    streamerSubtitle,
  } = useStreamMeta();
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const [showDetails, setShowDetails] = useState(false);
  const [now, setNow] = useState(Date.now);
  const latestActivity = Math.max(
    Date.parse(segment?.startTime ?? "") +
      Math.max(0, segment?.duration ?? 0) / 1e6 || -Infinity,
    Date.parse(livestream?.record.lastSeenAt ?? "") || -Infinity,
  );
  const liveUntil = latestActivity + 10_000;
  const isLive =
    isLiveMode &&
    livestream !== null &&
    !livestream.record.endedAt &&
    Number.isFinite(latestActivity) &&
    liveUntil > Math.max(now, Date.now());

  // A channel page can remain in live playback mode after its media stops.
  // Expire presence even if the websocket sends no further store updates.
  useEffect(() => {
    if (compact || !isLive) return;
    const timeout = setTimeout(
      () => setNow(Date.now()),
      liveUntil - Date.now(),
    );
    return () => clearTimeout(timeout);
  }, [compact, isLive, liveUntil]);

  const hasRights =
    contentRights?.license ||
    contentRights?.copyrightNotice ||
    contentRights?.copyrightYear ||
    contentRights?.creditLine;

  if (compact) {
    return <CompactStreamInfo />;
  }

  return (
    <View style={[layout.position.relative, px[4], py[3], gap.all[2]]}>
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.align.start,
          gap.all[3],
          { flexWrap: "wrap" },
        ]}
      >
        <View style={[gap.all[2], { flex: 1, minWidth: "50%" }]}>
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
          <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
            <Avatar
              src={avatarUri}
              name={profile ? formatHandle(profile) : undefined}
              size="lg"
              live={isLive}
              style={{ width: spacing[12], height: spacing[12] }}
            />

            <View style={{ flexShrink: 1, minWidth: 0 }}>
              <View
                style={[
                  layout.flex.row,
                  layout.flex.alignCenter,
                  gap.all[3],
                  { flexWrap: "wrap" },
                ]}
              >
                <Pressable
                  accessibilityRole="link"
                  accessibilityLabel={`Open profile for ${handleStr}`}
                  onPress={() => {
                    if (profile?.did) {
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
                {isLive && views !== null && (
                  <Text size="xs" color="muted" style={tabularNums}>
                    {t("watching-count", { count: views })}
                  </Text>
                )}
              </View>
              {streamerSubtitle ? (
                <Text numberOfLines={1} size="sm" color="muted">
                  {streamerSubtitle}
                </Text>
              ) : null}
            </View>
          </View>
        </View>

        {/* Stream actions remain available when a channel goes offline. */}
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            gap.all[2],
            { flexWrap: "wrap" },
          ]}
        >
          {did && profile && (
            <FollowButton streamerDID={profile.did} currentUserDID={did} />
          )}
          <ShareSheet />
          <KebabMenu />
          {isLiveMode && (
            <IconButton
              variant="secondary"
              size="sm"
              accessibilityLabel={showChat ? "Hide chat" : "Show chat"}
              onPress={() => setShowChat(!showChat)}
            >
              {showChat ? (
                <ChevronRight color={theme.colors.text2} size={spacing[4]} />
              ) : (
                <ChevronLeft color={theme.colors.text2} size={spacing[4]} />
              )}
            </IconButton>
          )}
        </View>
      </View>

      {/* Tags */}
      {tags && tags.length > 0 && (
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            gap.all[1],
            { flexWrap: "wrap" },
          ]}
        >
          {tags.map((tag) => (
            <View
              key={tag}
              style={[
                r.sm,
                px[1],
                {
                  backgroundColor: theme.colors.surface2,
                },
              ]}
            >
              <Text size="xs" color="muted">
                {tag}
              </Text>
            </View>
          ))}
        </View>
      )}

      {hasRights && (
        <View style={[gap.all[1]]}>
          <Pressable
            accessibilityRole="button"
            accessibilityState={{ expanded: showDetails }}
            onPress={() => setShowDetails(!showDetails)}
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              gap.all[1],
              py[1],
              {
                alignSelf: "flex-start",
                minHeight: theme.touchTargets.minimum,
              },
            ]}
          >
            <Text size="xs" color="muted">
              {t(showDetails ? "less-details" : "more-details")}
            </Text>
            {showDetails ? (
              <ChevronUp color={theme.colors.text2} size={spacing[3]} />
            ) : (
              <ChevronDown color={theme.colors.text2} size={spacing[3]} />
            )}
          </Pressable>
          {showDetails && contentRights && (
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

/** Shared derived stream metadata for both metadata layouts. */
function useStreamMeta() {
  const profile = useAuthor();
  const ls = useLivestreamStore((x) => x.livestream);
  const segment = useLivestreamStore((x) => x.segment);
  const mode = usePlayerStore((x) => x.mode);
  const did = useDID();
  const title = useTitle();
  const avatarUri = useAvatar();
  const views = useViews();

  const contentWarnings =
    (segment?.contentWarnings?.warnings as string[]) || [];
  const contentRights = segment?.contentRights;

  const activity = ls?.record.activity as
    | ((place.stream.defs.ActivityGame | place.stream.defs.ActivityLabel) & {
        $type?: string;
      })
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
  const isLive = mode === "live";

  // YouTube leads with the channel's display name; the handle is secondary.
  const displayName = profile?.displayName?.trim();
  const handleStr = profile ? formatHandleWithAt(profile) : "@user";
  const streamerName = displayName || handleStr;
  const streamerSubtitle = [displayName ? handleStr : null, activityLabel]
    .filter(Boolean)
    .join("  ·  ");

  return {
    profile,
    livestream: ls,
    segment,
    did,
    title,
    avatarUri,
    isLive,
    views,
    activityLabel,
    tags,
    contentWarnings,
    contentRights,
    handleStr,
    streamerName,
    streamerSubtitle,
  };
}

/**
 * Compact stream summary for mobile: status row, one-line title, handle row,
 * sitting transparently on the page background. Tapping the title or the
 * chevron expands tags, actions, and content notices; only that explicit
 * expansion changes the page geometry.
 */
function CompactStreamInfo() {
  const { theme, zero: z } = useTheme();
  const {
    profile,
    did,
    title,
    avatarUri,
    isLive,
    views,
    activityLabel,
    tags,
    contentWarnings,
    contentRights,
    handleStr,
  } = useStreamMeta();

  const [expanded, setExpanded] = useState(false);
  const expandHeight = useSharedValue(0);
  const contentHeight = useRef(0);

  const setExpandedAnimated = (next: boolean) => {
    setExpanded(next);
    expandHeight.value = withTiming(next ? contentHeight.current : 0, {
      duration: motion.base,
    });
  };

  const expandedStyle = useAnimatedStyle(() => ({
    height: expandHeight.value,
    opacity: Math.min(1, expandHeight.value / 48),
  }));

  const chevronColor = theme.colors.text2;
  const expandLabel = "Expand stream info";
  const collapseLabel = "Collapse stream info";

  return (
    <View style={[px[4], py[2], borders.bottom.width.thin, z.border.border]}>
      {/* single-line summary — tap to expand */}
      <View
        style={[layout.flex.row, layout.flex.alignCenter, gap.all[2], py[1]]}
      >
        <Avatar
          src={avatarUri}
          name={profile ? formatHandle(profile) : undefined}
          size="md"
          live={isLive}
        />
        {!expanded && (
          <Pressable
            onPress={() => setExpandedAnimated(!expanded)}
            accessibilityRole="button"
            accessibilityLabel="Stream info"
            style={{ flex: 1 }}
          >
            <Text
              numberOfLines={1}
              ellipsizeMode="tail"
              size="base"
              weight="semibold"
            >
              {title || handleStr}
            </Text>
            <Text numberOfLines={1} ellipsizeMode="tail" size="base">
              {handleStr}
            </Text>
          </Pressable>
        )}
        {expanded && (
          <Pressable
            accessibilityRole="link"
            accessibilityLabel={`Open profile for ${handleStr}`}
            onPress={() => {
              if (profile?.did) {
                Linking.openURL(
                  `https://bsky.app/profile/${formatHandle(profile)}`,
                );
              }
            }}
            style={{ flex: 1, minWidth: 0 }}
          >
            <Text numberOfLines={1} weight="semibold">
              {handleStr}
            </Text>
          </Pressable>
        )}
        {isLive ? (
          <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[1]]}>
            <View
              style={{
                width: 6,
                height: 6,
                borderRadius: 3,
                backgroundColor: statusColors.live,
              }}
            />
            {(views ?? 0) > 0 && (
              <Text size="sm" color="muted">
                {views}
              </Text>
            )}
          </View>
        ) : null}
        <Pressable
          onPress={() => setExpandedAnimated(!expanded)}
          accessibilityRole="button"
          accessibilityLabel={expanded ? collapseLabel : expandLabel}
        >
          {expanded ? (
            <ChevronUp size={18} color={chevronColor} />
          ) : (
            <ChevronDown size={18} color={chevronColor} />
          )}
        </Pressable>
      </View>

      {/* expanded details */}
      <Animated.View
        style={[expandedStyle, { overflow: "hidden" }]}
        pointerEvents={expanded ? "auto" : "none"}
      >
        <View
          style={[gap.all[2], pb[2]]}
          onLayout={(e) => {
            const h = e.nativeEvent.layout.height;
            contentHeight.current = h;
            if (expanded && Math.abs(expandHeight.value - h) > 1) {
              expandHeight.value = h;
            }
          }}
        >
          {title ? (
            <Text size="lg" weight="semibold">
              {title}
            </Text>
          ) : null}
          {did && profile && (
            <FollowButton streamerDID={profile.did} currentUserDID={did} />
          )}
          {tags && tags.length > 0 && (
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              contentContainerStyle={[gap.all[2], layout.flex.alignCenter]}
            >
              {activityLabel ? (
                <Text
                  numberOfLines={1}
                  size="sm"
                  color="muted"
                  style={{ flexShrink: 1 }}
                >
                  {activityLabel}
                </Text>
              ) : null}
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
          {(contentWarnings.length > 0 ||
            (contentRights && Object.keys(contentRights).length > 0)) && (
            <View>
              <ContentWarnings warnings={contentWarnings} compact={true} />
              {contentRights && (
                <ContentRights contentRights={contentRights} compact={true} />
              )}
            </View>
          )}
        </View>
      </Animated.View>

      {/* Atmosphere Conference inter-stream nav. TODO: remove after conf */}
      {profile?.handle &&
        ATMOCO_STREAMS.some((s) => s.handle === profile.handle) && (
          <AtMoCoNav currentHandle={profile.handle} />
        )}
    </View>
  );
}
