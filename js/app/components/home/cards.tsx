import { ContentWarnings } from "@streamplace/components";
import Viewers from "components/viewers";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { Image } from "react-native";
import { isWeb, Stack, Text, useMedia, View, XStack, YStack } from "tamagui";

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
  contentWarnings?: string[];
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
  contentWarnings = [],
}: StreamCardProps) => {
  const media = useMedia();

  const layoutHorizontal = horizontal;
  const { url } = useStreamplaceNode();

  // Define dynamic styles
  const borderRadius = 12;
  const contentPadding = 12;
  const avatarSize = 40;
  const livePillHeight = 30;
  const livePillPaddingHorizontal = 4;
  const categoryPillHeight = 16;
  const categoryPillPaddingHorizontal = 4;

  const MainContainer = layoutHorizontal ? XStack : YStack;
  const SubContainer = layoutHorizontal ? YStack : XStack;

  const verticalContentSectionHeight = avatarSize + 2 * contentPadding;
  const horizontalContentSectionWidth = avatarSize * 2 + contentPadding;

  return (
    <MainContainer
      flex={1}
      backgroundColor="$gray3"
      borderRadius={borderRadius}
      overflow="hidden"
      borderColor="#99889988"
      borderWidth={2}
      alignItems={layoutHorizontal ? "center" : "stretch"}
      hoverStyle={{
        backgroundColor: "$gray6",
      }}
    >
      {/* Thumbnail Section */}
      <Stack
        flex={layoutHorizontal ? 0 : undefined}
        minWidth={layoutHorizontal ? "67%" : "100%"}
        $gtXl={{
          minWidth: layoutHorizontal ? "65%" : "100%",
        }}
        $gtXxl={{
          minWidth: layoutHorizontal ? "62.5%" : "100%",
        }}
        // native seems to be unable to adjust widths properly?
        maxHeight={!isWeb ? "76.5%" : "100%"}
        borderRadius={borderRadius}
        overflow="hidden"
        position="relative"
        alignSelf={layoutHorizontal ? "auto" : "center"}
        backgroundColor="$gray6"
      >
        <Image
          source={{ uri: `${url}/${thumbnailUrl}`, width: 160, height: 90 }}
          style={{ width: "100%", height: "100%", aspectRatio: 16 / 9 }}
          resizeMode="contain"
        />
        {isLive && (
          <XStack
            position="absolute"
            top={contentPadding}
            right={contentPadding}
            backgroundColor="$background075"
            borderRadius={999}
            borderWidth={1}
            borderColor="#7774"
            paddingHorizontal={livePillPaddingHorizontal}
            height={livePillHeight}
            alignItems="center"
            justifyContent="center"
            gap={4}
          >
            <Viewers viewers={viewers} />
          </XStack>
        )}
      </Stack>

      {/* Content Section */}
      <SubContainer
        padding={contentPadding}
        alignItems={layoutHorizontal ? "flex-start" : "center"}
        justifyContent="flex-end"
        gap={contentPadding}
        height="unset"
        width={layoutHorizontal ? horizontalContentSectionWidth : "unset"}
        flex={1}
      >
        {/* Avatar */}
        <Stack
          width={avatarSize}
          height={avatarSize}
          borderRadius={avatarSize / 2}
          overflow="hidden"
          flexShrink={0}
        >
          {/* dynamically switching between these src crashes android */}
          {avatarUrl && (
            <View f={1} key="avatar">
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
        </Stack>

        {/* Text Content */}
        <YStack
          flex={1}
          justifyContent="space-around"
          alignItems="flex-start"
          gap={contentPadding / 4}
          width={layoutHorizontal ? "100%" : 0}
          minHeight={0}
          maxHeight="unset"
          zIndex={12}
        >
          {title && (
            <Text
              fontSize={16}
              color="$color"
              fontWeight="400"
              numberOfLines={1}
              ellipsizeMode="tail"
            >
              {title}
            </Text>
          )}
          {streamerName && (
            <Text
              fontSize={14}
              color="$color"
              fontWeight="400"
              numberOfLines={1}
              ellipsizeMode="tail"
            >
              @{streamerName}
            </Text>
          )}
          <ContentWarnings warnings={contentWarnings} compact={true} />
          {category.length > 0 && (
            <XStack flexWrap="wrap" gap={4} alignItems="center">
              {category.map((cat, index) => (
                <Stack
                  key={index}
                  backgroundColor="$background075"
                  borderRadius={999}
                  paddingHorizontal={categoryPillPaddingHorizontal}
                  height={categoryPillHeight}
                  alignSelf="flex-start"
                  justifyContent="center"
                >
                  <Text
                    fontSize={12}
                    color="$white075"
                    fontWeight="400"
                    numberOfLines={1}
                    ellipsizeMode="tail"
                    paddingHorizontal={3}
                  >
                    {cat}
                  </Text>
                </Stack>
              ))}
            </XStack>
          )}
        </YStack>
      </SubContainer>
    </MainContainer>
  );
};

export default StreamCard;
