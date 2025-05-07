import React from "react";
import { Image } from "react-native";
import { Stack, Text, XStack, YStack, useMedia } from "tamagui";
import Viewers from "components/viewers";

export type StreamCardSize = "xs" | "sm" | "md" | "lg" | "xl";

interface StreamCardProps {
  size?: StreamCardSize;
  horizontal?: boolean;
  thumbnailUrl: string;
  avatarUrl: string;
  title?: string;
  streamerName?: string;
  viewers?: number;
  category: string[];
  isLive?: boolean;
}

const StreamCard: React.FC<StreamCardProps> = ({
  size = "sm",
  horizontal = false,
  thumbnailUrl,
  avatarUrl,
  title,
  streamerName,
  viewers = 0,
  category = [],
  isLive = true,
}) => {
  const media = useMedia();

  // Determine layout based on screen size
  const isLargeScreen = media.gtMd;
  const layoutHorizontal = horizontal;

  // Define dynamic styles
  const borderRadius = 12;
  const contentPadding = 8;
  const avatarSize = 40;
  const livePillHeight = 24;
  const livePillPaddingHorizontal = 6;
  const categoryPillHeight = 16;
  const categoryPillPaddingHorizontal = 4;

  const MainContainer = layoutHorizontal ? XStack : YStack;

  return (
    <MainContainer
      flex={1}
      backgroundColor="$accentBackground"
      borderRadius={borderRadius}
      overflow="hidden"
      alignItems={layoutHorizontal ? "center" : "stretch"}
      hoverStyle={{
        backgroundColor: "$purple6Dark",
      }}
    >
      {/* Thumbnail Section */}
      <Stack
        flex={layoutHorizontal ? 0 : undefined}
        width={layoutHorizontal ? "40%" : "100%"}
        aspectRatio={16 / 9}
        borderRadius={borderRadius}
        overflow="hidden"
        position="relative"
        alignSelf={layoutHorizontal ? "auto" : "center"}
      >
        <Image
          source={{ uri: thumbnailUrl }}
          style={{ width: "100%", height: "100%" }}
          resizeMode="cover"
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
            shadowColor="$background075"
            shadowOffset={{ width: 0, height: 2 }}
            shadowOpacity={0.25}
            shadowRadius={4}
          >
            <Viewers viewers={viewers} />
          </XStack>
        )}
      </Stack>

      {/* Content Section */}
      <XStack
        flex={1}
        padding={contentPadding}
        alignItems="center"
        gap={contentPadding}
      >
        {/* Avatar */}
        <Stack
          width={avatarSize}
          height={avatarSize}
          borderRadius={avatarSize / 2}
          overflow="hidden"
        >
          <Image
            source={{ uri: avatarUrl }}
            style={{ width: "100%", height: "100%" }}
            resizeMode="cover"
          />
        </Stack>

        {/* Text Content */}
        <YStack
          flex={1}
          justifyContent="space-between"
          alignItems="flex-start"
          gap={contentPadding / 2}
          style={{ width: "100%", height: "100%" }}
        >
          {title && (
            <Text
              fontSize={16}
              color="white"
              fontWeight="400"
              numberOfLines={1}
              ellipsizeMode="tail"
              style={{ flexShrink: 1 }}
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
              style={{ flexShrink: 1 }}
            >
              {streamerName}
            </Text>
          )}
          {category.length > 0 && (
            <XStack flexWrap="wrap" gap={4}>
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
                  <Text fontSize={12} color="$gray5" fontWeight="400">
                    {cat}
                  </Text>
                </Stack>
              ))}
            </XStack>
          )}
        </YStack>
      </XStack>
    </MainContainer>
  );
};

export default StreamCard;
