import {
  Avatar,
  ContentRights,
  formatActivity,
  IconButton,
  layout,
  ShareSheet,
  Text,
  useAuthor,
  useAvatar,
  useDID,
  useLivestreamStore,
  useTheme,
  useTitle,
  useTranslation,
  useViews,
  zero,
} from "@streamplace/components";
import {
  motion,
  spacing,
  tabularNums,
} from "@streamplace/components/src/lib/theme/tokens";
import { formatHandle, formatHandleWithAt } from "@streamplace/core";
import AQLink from "components/aqlink";
import FollowButton from "components/follow-button";
import { useLiveStatus } from "hooks/useLiveStatus";
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
} from "lucide-react-native";
import { useRef, useState } from "react";
import { Linking, Pressable, View } from "react-native";
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
  return compact ? (
    <CompactStreamInfo />
  ) : (
    <FullStreamInfo setShowChat={setShowChat} showChat={showChat} />
  );
}

function FullStreamInfo({
  setShowChat,
  showChat,
}: {
  setShowChat: (show: boolean) => void;
  showChat: boolean;
}) {
  const {
    profile,
    did,
    title,
    avatarUri,
    isLiveMode,
    isLive,
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

  const hasRights =
    contentRights?.license ||
    contentRights?.copyrightNotice ||
    contentRights?.copyrightYear ||
    contentRights?.creditLine;

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
            <View
              style={[
                layout.flex.center,
                { width: spacing[12], height: spacing[12] },
              ]}
            >
              <Avatar
                src={avatarUri}
                name={profile ? formatHandle(profile) : undefined}
                size="lg"
                live={isLive}
              />
            </View>

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
          {profile && (
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
                r.full,
                px[2],
                {
                  backgroundColor: theme.colors.surface3,
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
  const { livestream: ls, segment, mode, isLive } = useLiveStatus();
  const did = useDID();
  const title = useTitle();
  const avatarUri = useAvatar();
  const views = useViews();
  const presence = useLivestreamStore((x) =>
    x.websocketConnected ? x.presence : null,
  );
  const isLiveMode = mode === "live";

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

  const displayName = profile?.displayName?.trim();
  const handleStr = profile ? formatHandleWithAt(profile) : "@user";
  const streamerName = displayName || handleStr;
  const streamerSubtitle = [displayName ? handleStr : null, activityLabel]
    .filter(Boolean)
    .join("  ·  ");

  return {
    profile,
    did,
    title,
    avatarUri,
    isLive,
    isLiveMode,
    views,
    presence,
    activityLabel,
    tags,
    contentRights,
    handleStr,
    streamerName,
    streamerSubtitle,
  };
}

/**
 * The phone summary expands from a title and handle into creator identity,
 * the full title, wrapping tags and optional stream rights.
 */
function CompactStreamInfo() {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const {
    profile,
    did,
    title,
    avatarUri,
    isLive,
    presence,
    activityLabel,
    tags,
    contentRights,
    handleStr,
    streamerName,
  } = useStreamMeta();

  let metadataTags = tags;
  if (activityLabel) {
    metadataTags = [activityLabel];
    for (const tag of tags ?? []) {
      if (tag !== activityLabel) metadataTags.push(tag);
    }
  }

  const [expanded, setExpanded] = useState(false);
  const [showDetails, setShowDetails] = useState(false);
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
    opacity: Math.min(1, expandHeight.value / theme.spacing[12]),
  }));

  const chevronColor = theme.colors.text2;
  const expandLabel = "Expand stream info";
  const collapseLabel = "Collapse stream info";
  const countLabel =
    !isLive && presence !== null
      ? t("here-count", { count: presence })
      : undefined;
  const hasRights =
    contentRights?.license ||
    contentRights?.copyrightNotice ||
    contentRights?.copyrightYear ||
    contentRights?.creditLine;

  return (
    <View
      style={[
        px[4],
        py[2],
        borders.bottom.width.thin,
        { borderColor: theme.colors.border },
      ]}
    >
      {/* single-line summary — tap to expand */}
      <View
        style={[layout.flex.row, layout.flex.alignCenter, gap.all[2], py[1]]}
      >
        <View
          style={[
            layout.flex.center,
            {
              width: theme.touchTargets.minimum,
              height: theme.touchTargets.minimum,
            },
          ]}
        >
          <Avatar
            src={avatarUri}
            name={profile ? formatHandle(profile) : undefined}
            size="md"
            live={isLive}
          />
        </View>
        {!expanded && (
          <Pressable
            onPress={() => setExpandedAnimated(!expanded)}
            accessibilityRole="button"
            accessibilityLabel={
              countLabel ? `Stream info, ${countLabel}` : "Stream info"
            }
            style={{ flex: 1, minWidth: 0 }}
          >
            <Text
              numberOfLines={1}
              ellipsizeMode="tail"
              size="sm"
              weight="semibold"
            >
              {title || handleStr}
            </Text>
            <View style={[layout.flex.row, layout.flex.alignCenter]}>
              <Text
                numberOfLines={1}
                ellipsizeMode="tail"
                size="xs"
                color="muted"
                style={{ flexShrink: 1 }}
              >
                {handleStr}
              </Text>
              {countLabel !== undefined && (
                <Text
                  size="xs"
                  color="muted"
                  style={[tabularNums, { flexShrink: 0 }]}
                >
                  {" · "}
                  {countLabel}
                </Text>
              )}
            </View>
          </Pressable>
        )}
        {expanded && (
          <View style={{ flex: 1, minWidth: 0 }}>
            <View
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                gap.all[2],
                { flexWrap: "wrap" },
              ]}
            >
              <Pressable
                accessibilityRole="link"
                accessibilityLabel={`Open profile for ${handleStr}`}
                onPress={() =>
                  profile?.did &&
                  Linking.openURL(
                    `https://bsky.app/profile/${formatHandle(profile)}`,
                  )
                }
                style={{ flexShrink: 1, minWidth: 0 }}
              >
                <Text numberOfLines={1} size="sm" weight="semibold">
                  {streamerName}
                </Text>
              </Pressable>
              {countLabel !== undefined && (
                <Text size="xs" color="muted" style={tabularNums}>
                  {countLabel}
                </Text>
              )}
            </View>
            {streamerName !== handleStr && (
              <Text numberOfLines={1} size="xs" color="muted">
                {handleStr}
              </Text>
            )}
          </View>
        )}
        {profile && (
          <FollowButton
            streamerDID={profile.did}
            currentUserDID={did}
            iconOnly
          />
        )}
        <IconButton
          onPress={() => setExpandedAnimated(!expanded)}
          accessibilityLabel={expanded ? collapseLabel : expandLabel}
          accessibilityState={{ expanded }}
        >
          {expanded ? (
            <ChevronUp size={spacing[4]} color={chevronColor} />
          ) : (
            <ChevronDown size={spacing[4]} color={chevronColor} />
          )}
        </IconButton>
      </View>

      {/* expanded details */}
      <Animated.View
        style={[expandedStyle, { overflow: "hidden" }]}
        pointerEvents={expanded ? "auto" : "none"}
        accessibilityElementsHidden={!expanded}
        importantForAccessibility={expanded ? "auto" : "no-hide-descendants"}
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
            <Text size="base" weight="semibold">
              {title}
            </Text>
          ) : null}
          {metadataTags && metadataTags.length > 0 && (
            <View
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                gap.all[1],
                { flexWrap: "wrap" },
              ]}
            >
              {metadataTags.map((tag) => (
                <View
                  key={tag}
                  style={[
                    r.full,
                    px[2],
                    {
                      backgroundColor: theme.colors.surface3,
                      maxWidth: "100%",
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
            <View>
              <Pressable
                accessibilityRole="button"
                accessibilityState={{ expanded: showDetails }}
                onPress={() => setShowDetails(!showDetails)}
                style={[
                  layout.flex.row,
                  layout.flex.alignCenter,
                  gap.all[1],
                  {
                    minHeight: theme.touchTargets.minimum,
                    alignSelf: "flex-start",
                  },
                ]}
              >
                <Text size="xs" color="muted">
                  {t("stream-details")}
                </Text>
                {showDetails ? (
                  <ChevronUp size={spacing[3]} color={chevronColor} />
                ) : (
                  <ChevronDown size={spacing[3]} color={chevronColor} />
                )}
              </Pressable>
              {showDetails && contentRights && (
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
