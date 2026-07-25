import { LiquidGlassView } from "@callstack/liquid-glass";
import {
  hexToRgba,
  PlayerUI,
  Text,
  useTheme,
  zero,
} from "@streamplace/components";
import { Image } from "expo-image";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useMemo, useState } from "react";
import { Platform, View } from "react-native";

export type StreamCardSize = "xs" | "sm" | "md" | "lg" | "xl";

const displayTag = (tag: string): string => {
  // could be top level but we want to make sure RN polyfill runs first
  const langNames = new Intl.DisplayNames(["en"], { type: "language" });
  if (tag.startsWith("lang:")) {
    try {
      return langNames.of(tag.slice(5)) ?? tag;
    } catch {
      return tag;
    }
  }
  return tag;
};

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

  const tagItems = tags.length > 0 ? tags : category;
  const tagsKey = tagItems.join(",");

  const [rowWidth, setRowWidth] = useState(0);
  const [itemWidths, setItemWidths] = useState<Record<string, number>>({});

  useEffect(() => {
    setItemWidths({});
  }, [activity, tagsKey]);

  const visibleTagCount = useMemo(() => {
    if (rowWidth === 0) return tagItems.length;
    const activityW = activity ? (itemWidths["activity"] ?? 0) : 0;
    let used = activityW;
    let count = 0;
    for (let i = 0; i < tagItems.length; i++) {
      const w = itemWidths[`tag-${i}`];
      if (w === undefined) {
        count++;
        continue;
      }
      const gap = used > 0 ? 6 : 0;
      if (used + gap + w <= rowWidth) {
        used += gap + w;
        count++;
      } else {
        break;
      }
    }
    return count;
  }, [rowWidth, itemWidths, tagItems, activity]);

  // Define dynamic styles
  const borderRadius = 12;
  const contentPaddingHoriz = 12;
  const contentPaddingVertical = 2.65;
  const avatarSize = 40;
  const livePillHeight = inMobileMode ? 20 : 30;
  const livePillPaddingHorizontal = inMobileMode ? 2 : 4;

  const contentSectionHeight = avatarSize + 2 * contentPaddingVertical;
  const contentSectionWidth = avatarSize * 2 + contentPaddingHoriz;

  return (
    <LiquidGlassView
      interactive
      style={[
        inMobileMode ? { alignSelf: "stretch" } : zero.flex.values[1],
        {
          borderCurve: "continuous",
          backgroundColor: theme.colors.muted,
          borderRadius,
          overflow: "hidden",
          borderColor: theme.colors.mutedForeground + 80,
          borderWidth: isWeb ? 1 : 0,
          alignItems: layoutHorizontal ? "center" : "stretch",
          flexDirection: layoutHorizontal ? "row" : "column",
        },
      ]}
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
            // native seems to be unable to adjust widths properly?
            maxHeight: !isWeb ? (inMobileMode ? "100%" : "76.5%") : "100%",
            position: "relative",
            alignSelf: layoutHorizontal ? "auto" : "center",
            backgroundColor: theme.colors.card,
          },
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
        />
        {isLive && (
          <View
            style={[
              {
                position: "absolute",
                top: contentPaddingVertical * 2,
                right: contentPaddingVertical * 2,
                backgroundColor: "rgba(0, 0, 0, 0.75)",
                borderRadius: 999,
                borderWidth: 1,
                borderColor: "rgba(119, 119, 119, 0.25)",
                paddingHorizontal: livePillPaddingHorizontal,
                height: livePillHeight,
                alignItems: "center",
                justifyContent: "center",
                gap: 0,
                flexDirection: "row",
              },
            ]}
          >
            <PlayerUI.DehydratedViewers
              viewers={viewers}
              size={inMobileMode ? "sm" : "md"}
            />
          </View>
        )}
      </View>

      {/* Content */}
      <View
        style={[
          {
            paddingHorizontal: contentPaddingHoriz,
            paddingVertical: contentPaddingVertical,
            alignItems: layoutHorizontal ? "flex-start" : "center",
            justifyContent: "flex-end",
            gap: contentPaddingHoriz,
            width: layoutHorizontal ? contentSectionWidth : "auto",
            flex: 1,
            flexDirection: layoutHorizontal ? "column" : "row",
          },
        ]}
      >
        {/* Avatar */}
        {showAvatar && (
          <View
            style={[
              {
                width: avatarSize,
                height: avatarSize,
                marginVertical: layoutHorizontal
                  ? 0
                  : contentPaddingVertical * 4,
                borderRadius: avatarSize / 2,
                overflow: "hidden",
                flexShrink: 0,
              },
            ]}
          >
            {/* dynamically switching between these src crashes android */}
            {avatarUrl && (
              <View style={[zero.flex.values[1]]} key="avatar">
                <Image
                  key="avatar"
                  source={{
                    uri: avatarUrl,
                  }}
                  style={{ width: "100%", height: "100%" }}
                  contentFit="cover"
                />
              </View>
            )}
            {!avatarUrl && (
              <View key="avatar-placeholder">
                <Image
                  key="avatar"
                  source={require("./../../assets/images/goose.png")}
                  style={{ width: "100%", height: "100%" }}
                  contentFit="cover"
                />
              </View>
            )}
          </View>
        )}

        {/* Text content */}
        <View
          style={[
            zero.flex.values[1],
            { justifyContent: "center" },
            { alignItems: "flex-start" },
            {
              gap: contentPaddingHoriz / 4,
              width: layoutHorizontal ? "100%" : 0,
              minHeight: 0,
              zIndex: 12,
            },
          ]}
        >
          {title && (
            <Text
              style={[
                {
                  lineHeight: 16,
                },
              ]}
              size={showAvatar ? "base" : "base"}
              numberOfLines={1}
              ellipsizeMode="tail"
            >
              {title}
            </Text>
          )}
          {streamerName && (
            <Text
              size="sm"
              style={[
                {
                  lineHeight: 16,
                },
              ]}
              numberOfLines={1}
              ellipsizeMode="tail"
              leading="tight"
            >
              @{streamerName}
            </Text>
          )}
          {((activity && category.length > 0) || tags.length > 0) && (
            <View
              style={{
                flexWrap: inMobileMode ? "wrap" : "nowrap",
                gap: inMobileMode ? 4 : 8,
                alignItems: "center",
                alignSelf: "stretch",
                flexDirection: "row",
                overflow: "hidden",
                maxHeight: inMobileMode ? 40 : undefined,
              }}
              onLayout={(e) => {
                const width = e.nativeEvent?.layout?.width;
                if (!width) {
                  return;
                }
                setRowWidth(width);
              }}
            >
              {activity && (
                <Text
                  size="sm"
                  style={{ flexShrink: 0 }}
                  color={hexToRgba(theme.colors.accentForeground, 0.85)}
                  numberOfLines={1}
                  ellipsizeMode="tail"
                  onLayout={(e) => {
                    const width = e.nativeEvent?.layout?.width;
                    if (!width) {
                      return;
                    }
                    setItemWidths((prev) => ({
                      ...prev,
                      activity: width,
                    }));
                  }}
                >
                  {activity}
                </Text>
              )}
              {(inMobileMode
                ? tagItems
                : tagItems.slice(0, visibleTagCount)
              ).map((cat, index) => (
                <View
                  key={index}
                  style={[
                    zero.r.full,
                    {
                      borderWidth: 1,
                      borderColor: theme.colors.border,
                      backgroundColor: hexToRgba(theme.colors.secondary, 0.3),
                      paddingHorizontal: 8,
                      flexShrink: 0,
                    },
                  ]}
                  onLayout={(e) => {
                    const width = e.nativeEvent?.layout?.width;
                    if (!width) {
                      return;
                    }
                    setItemWidths((prev) => ({
                      ...prev,
                      [`tag-${index}`]: width,
                    }));
                  }}
                >
                  <Text
                    size={inMobileMode ? "xs" : "sm"}
                    color={hexToRgba(theme.colors.primaryForeground, 0.85)}
                    numberOfLines={1}
                    ellipsizeMode="tail"
                  >
                    {displayTag(cat)}
                  </Text>
                </View>
              ))}
            </View>
          )}
        </View>
      </View>
    </LiquidGlassView>
  );
};

export default StreamCard;
