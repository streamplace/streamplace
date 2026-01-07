import { PlayerUI, Text, useTheme, zero } from "@streamplace/components";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { Image, Platform, View } from "react-native";

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
  const categoryPillHeight = 16;
  const categoryPillPaddingHorizontal = 4;

  const verticalContentSectionHeight = avatarSize + 2 * contentPadding;
  const horizontalContentSectionWidth = avatarSize * 2 + contentPadding;

  return (
    <View
      style={[
        zero.flex.values[1],
        {
          backgroundColor: theme.colors.muted,
          borderRadius,
          overflow: "hidden",
          borderColor: theme.colors.mutedForeground + 80,
          borderWidth: 2,
          alignItems: layoutHorizontal ? "center" : "stretch",
          flexDirection: layoutHorizontal ? "row" : "column",
        },
      ]}
    >
      {/* Thumbnail Section */}
      <View
        style={[
          {
            flex: layoutHorizontal ? 0 : undefined,
            minWidth: layoutHorizontal ? "63%" : "100%",
            // native seems to be unable to adjust widths properly?
            maxHeight: !isWeb ? "76.5%" : "100%",
            borderRadius,
            overflow: "hidden",
            position: "relative",
            alignSelf: layoutHorizontal ? "auto" : "center",
            backgroundColor: theme.colors.card,
          },
        ]}
      >
        <Image
          source={{ uri: `${url}/${thumbnailUrl}`, width: 160, height: 90 }}
          style={{ width: "100%", height: "100%", aspectRatio: 16 / 9 }}
          resizeMode="contain"
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

      {/* Content Section */}
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
                resizeMode="cover"
              />
            </View>
          )}
          {!avatarUrl && (
            <View key="avatar-placeholder">
              <Image
                key="avatar"
                source={require("./../../assets/images/goose.png")}
                style={{ width: "100%", height: "100%" }}
                resizeMode="cover"
              />
            </View>
          )}
        </View>

        {/* Text Content */}
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
            >
              @{streamerName}
            </Text>
          )}
          {category.length > 0 && (
            <View
              style={[
                {
                  flexWrap: "wrap",
                  gap: 4,
                  alignItems: "center",
                  flexDirection: "row",
                },
              ]}
            >
              {category.map((cat, index) => (
                <View
                  key={index}
                  style={[
                    {
                      backgroundColor: "rgba(0, 0, 0, 0.75)",
                      borderRadius: 999,
                      paddingHorizontal: categoryPillPaddingHorizontal,
                      height: categoryPillHeight,
                      alignSelf: "flex-start",
                      justifyContent: "center",
                    },
                  ]}
                >
                  <Text
                    style={[
                      {
                        fontSize: 12,
                        color: "rgba(255, 255, 255, 0.75)",
                        fontWeight: "400",
                        paddingHorizontal: 3,
                      },
                    ]}
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
    </View>
  );
};

export default StreamCard;
