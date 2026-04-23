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
}

const StreamCard = ({
  size = "sm",
  horizontal = false,
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
  const { url } = useStreamplaceNode();
  const { theme } = useTheme();
  const isWeb = Platform.OS === "web";

  // Define dynamic styles
  const borderRadius = 12;
  const contentPadding = 12;
  const avatarSize = 40;
  const livePillHeight = 30;
  const livePillPaddingHorizontal = 4;

  const verticalContentSectionHeight = avatarSize + 2 * contentPadding;
  const horizontalContentSectionWidth = avatarSize * 2 + contentPadding;

  return (
    <LiquidGlassView
      interactive
      style={[
        zero.flex.values[1],
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
            minWidth: layoutHorizontal ? "63%" : "100%",
            // native seems to be unable to adjust widths properly?
            maxHeight: !isWeb ? "76.5%" : "100%",
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
                top: contentPadding,
                right: contentPadding,
                backgroundColor: "rgba(0, 0, 0, 0.75)",
                borderRadius: 999,
                borderWidth: 1,
                borderColor: "rgba(119, 119, 119, 0.25)",
                paddingHorizontal: livePillPaddingHorizontal,
                height: livePillHeight,
                alignItems: "center",
                justifyContent: "center",
                gap: 4,
                flexDirection: "row",
              },
            ]}
          >
            <PlayerUI.DehydratedViewers viewers={viewers} />
          </View>
        )}
      </View>

      {/* Content */}
      <View
        style={[
          {
            padding: contentPadding,
            alignItems: layoutHorizontal ? "flex-start" : "center",
            justifyContent: "flex-end",
            gap: contentPadding,
            width: layoutHorizontal ? horizontalContentSectionWidth : "auto",
            flex: 1,
            flexDirection: layoutHorizontal ? "column" : "row",
          },
        ]}
      >
        {/* Avatar */}
        <View
          style={[
            {
              width: avatarSize,
              height: avatarSize,
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

        {/* Text content */}
        <View
          style={[
            zero.flex.values[1],
            { justifyContent: "space-around" },
            { alignItems: "flex-start" },
            {
              gap: contentPadding / 4,
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
                flexWrap: "wrap",
                gap: 6,
                alignItems: "center",
                flexDirection: "row",
              }}
            >
              {activity && (
                <Text
                  size="sm"
                  style={{ color: theme.colors.ring }}
                  numberOfLines={1}
                  ellipsizeMode="tail"
                >
                  {activity}
                </Text>
              )}
              {(tags.length > 0 ? tags : category).map((cat, index) => (
                <View
                  key={index}
                  style={[
                    zero.r.full,
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
                    numberOfLines={1}
                    ellipsizeMode="tail"
                  >
                    {cat}
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
