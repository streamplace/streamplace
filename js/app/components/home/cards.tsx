import {
  Avatar,
  LiveBadge,
  Text,
  useTheme,
  zero,
} from "@streamplace/components";
import {
  borderRadius,
  motion,
  spacing,
} from "@streamplace/components/src/lib/theme/tokens";
import { Image } from "expo-image";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useState } from "react";
import { Platform, View } from "react-native";

export type StreamCardSize = "xs" | "sm" | "md" | "lg" | "xl";

interface StreamCardProps {
  size?: StreamCardSize;
  horizontal?: boolean;
  thumbnailUrl: string;
  avatarUrl?: string;
  title?: string;
  streamerName?: string;
  viewers?: number;
  category: string[];
  activity?: string;
  tags?: string[];
  isLive?: boolean;
  showAvatar?: boolean;
}

// 11400 -> "11K", 1350 -> "1.3K", 942 -> "942"
function formatViewers(n: number): string {
  if (n >= 1_000_000)
    return `${(n / 1_000_000).toFixed(n < 10_000_000 ? 1 : 0)}M`;
  if (n >= 10_000) return `${Math.round(n / 1000)}K`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return `${n}`;
}

/**
 * YouTube-grammar stream card: a rounded 16:9 thumbnail with a LIVE chip
 * bottom-right, then a clean three-line meta stack — title (16px medium,
 * 2 lines), handle, activity · viewers. No chips, no noise.
 */
const StreamCard = ({
  size = "sm",
  horizontal = false,
  showAvatar = true,
  thumbnailUrl,
  avatarUrl,
  title,
  streamerName,
  viewers = 0,
  category = [],
  activity,
  tags = [],
  isLive = true,
}: StreamCardProps) => {
  const layoutHorizontal = horizontal;
  const inMobileMode = horizontal && !showAvatar;
  const { url } = useStreamplaceNode();
  const { theme } = useTheme();
  const isWeb = Platform.OS === "web";
  const [hovered, setHovered] = useState(false);

  const webTransition = isWeb
    ? ({
        transitionDuration: `${motion.fast}ms`,
        transitionTimingFunction: motion.easingCss,
        transitionProperty: "transform, border-color",
      } as any)
    : null;

  const watchingLine = [
    activity,
    isLive && viewers > 0 ? `${formatViewers(viewers)} watching` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <View
      style={[
        inMobileMode ? { alignSelf: "stretch" } : zero.flex.values[1],
        {
          alignItems: layoutHorizontal ? "center" : "stretch",
          flexDirection: layoutHorizontal ? "row" : "column",
          gap: spacing[3],
        },
      ]}
      {...(isWeb && {
        onPointerEnter: () => setHovered(true),
        onPointerLeave: () => setHovered(false),
      })}
    >
      {/* Thumbnail */}
      <View
        style={[
          {
            flex: layoutHorizontal ? 0 : undefined,
            minWidth: layoutHorizontal
              ? inMobileMode
                ? "40%"
                : "63%"
              : "100%",
            maxWidth: layoutHorizontal ? "40%" : undefined,
            position: "relative",
            alignSelf: layoutHorizontal ? "auto" : "center",
            backgroundColor: theme.colors.surface1,
            borderRadius: borderRadius.lg,
            borderCurve: "continuous",
            borderWidth: 1,
            borderColor: hovered
              ? theme.colors.borderStrong
              : theme.colors.borderSubtle,
            overflow: "hidden",
            transform: [{ scale: hovered ? 1.02 : 1 }],
          },
          webTransition,
        ]}
      >
        <Image
          source={{ uri: `${url}/${thumbnailUrl}` }}
          style={{
            width: "100%",
            height: "100%",
            aspectRatio: 16 / 9,
          }}
          contentFit="contain"
          transition={100}
        />
        {isLive && (
          <View
            style={{
              position: "absolute",
              bottom: spacing[2],
              right: spacing[2],
            }}
          >
            <LiveBadge />
          </View>
        )}
      </View>

      {/* Meta: avatar + three-line stack */}
      <View
        style={[
          {
            paddingHorizontal: layoutHorizontal ? 0 : spacing[1],
            alignItems: "flex-start",
            gap: spacing[3],
            flex: 1,
            flexDirection: "row",
            width: layoutHorizontal ? undefined : "auto",
          },
        ]}
      >
        {showAvatar && (
          <Avatar src={avatarUrl} name={streamerName} size="lg" live={isLive} />
        )}

        <View
          style={[
            zero.flex.values[1],
            {
              alignItems: "flex-start",
              gap: 2,
              minHeight: 0,
              minWidth: 0,
            },
          ]}
        >
          {title && (
            <Text
              size={inMobileMode ? "base" : "lg"}
              weight="medium"
              numberOfLines={2}
              ellipsizeMode="tail"
              style={{ alignSelf: "stretch" }}
            >
              {title}
            </Text>
          )}
          {streamerName && (
            <Text
              size="sm"
              numberOfLines={1}
              ellipsizeMode="tail"
              style={{ color: theme.colors.text3 }}
            >
              @{streamerName}
            </Text>
          )}
          {watchingLine.length > 0 && (
            <Text
              size="sm"
              numberOfLines={1}
              ellipsizeMode="tail"
              tabular
              style={{ color: theme.colors.text3 }}
            >
              {watchingLine}
            </Text>
          )}
        </View>
      </View>
    </View>
  );
};

export default StreamCard;
