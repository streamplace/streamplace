import Viewers from "components/viewers";
import React from "react";
import { Image } from "react-native"; // Using RN Image
import { Stack, Text, XStack, YStack } from "tamagui"; // Assuming Tamagui is set up

// Define size configurations
const sizeConfig = {
  xs: {
    // Vertical dimensions
    cardVerticalWidth: 180,
    gapVertical: 0,
    // Horizontal dimensions (reused from previous)
    cardHorizontalWidth: 280,
    gapHorizontal: 0,
    // Common dimensions
    thumbnail: { width: 180, height: 101, borderRadius: 8 }, // 16:9 aspect scaled from sm thumbnail width
    livePill: { width: 40, height: 16, paddingHorizontal: 3, gap: 1 },
    liveTextSize: 10,
    avatarSize: 24,
    contentPadding: 4,
    titleFontSize: 10,
    streamerFontSize: 10,
    categoryPill: { width: 40, height: 9, paddingHorizontal: 1 },
    categoryFontSize: 7,
  },
  sm: {
    // Based on provided CSS for Vertical layout, adjusted for horizontal
    // Vertical dimensions
    cardVerticalWidth: 300, // Based on original thumbnail width
    gapVertical: 0, // Added gap between thumbnail and content
    // Horizontal dimensions (reused from previous)
    cardHorizontalWidth: 580, // Sum of thumbnail + content width (approx)
    gapHorizontal: 2, // Based on board-23881f124a3d padding
    // Common dimensions
    thumbnail: { width: 300, height: 169, borderRadius: 11 }, // 16:9 aspect based on original
    livePill: { width: 66, height: 24, paddingHorizontal: 5, gap: 2 },
    liveTextSize: 16,
    avatarSize: 42,
    contentPadding: 6, // Based on board-23881f124a3d padding
    titleFontSize: 14, // Based on text size
    streamerFontSize: 14, // Based on text size
    categoryPill: { width: 75, height: 13, paddingHorizontal: 2 },
    categoryFontSize: 10, // Based on text size
  },
  md: {
    // vertical dimensions
    cardVerticalWidth: 400,
    gapVertical: 0,
    // horizontal dimensions
    cardHorizontalWidth: 700,
    gapHorizontal: 2,
    // common dimensions
    thumbnail: { width: 400, height: 225, borderRadius: 14 },
    livePill: { width: 80, height: 30, paddingHorizontal: 6, gap: 3 },
    liveTextSize: 16,
    avatarSize: 50,
    contentPadding: 12,
    titleFontSize: 16,
    streamerFontSize: 16,
    categoryPill: { width: 90, height: 16, paddingHorizontal: 3 },
    categoryFontSize: 12,
  },
  lg: {
    // Vertical dimensions
    cardVerticalWidth: 500,
    gapVertical: 16,
    cardHorizontalWidth: 900,
    gapHorizontal: 4,
    // Common dimensions
    thumbnail: { width: 500, height: 281, borderRadius: 18 },
    livePill: { width: 100, height: 36, paddingHorizontal: 7, gap: 4 },
    liveTextSize: 20,
    avatarSize: 60,
    contentPadding: 10,
    titleFontSize: 18,
    streamerFontSize: 18,
    categoryPill: { width: 110, height: 18, paddingHorizontal: 4 },
    categoryFontSize: 14,
  },
  xl: {
    // Vertical dimensions
    cardVerticalWidth: 600,
    gapVertical: 20,
    // Horizontal dimensions
    cardHorizontalWidth: 1100,
    gapHorizontal: 6,
    // Common dimensions
    thumbnail: { width: 600, height: 338, borderRadius: 22 },
    livePill: { width: 120, height: 42, paddingHorizontal: 8, gap: 5 },
    liveTextSize: 24,
    avatarSize: 70,
    contentPadding: 12,
    titleFontSize: 20,
    streamerFontSize: 20,
    categoryPill: { width: 130, height: 20, paddingHorizontal: 5 },
    categoryFontSize: 16,
  },
};

type StreamCardSize = "xs" | "sm" | "md" | "lg" | "xl";

interface StreamCardProps {
  size?: StreamCardSize;
  horizontal?: boolean; // New prop
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
  horizontal = false, // Default to vertical
  thumbnailUrl,
  avatarUrl,
  title,
  streamerName,
  viewers = 0,
  category = [],
  isLive = true,
}) => {
  const config = sizeConfig[size];

  // Main container - YStack for vertical, XStack for horizontal
  const MainContainer = horizontal ? XStack : YStack;

  return (
    <MainContainer
      width={horizontal ? config.cardHorizontalWidth : config.cardVerticalWidth}
      backgroundColor="$accentBackground" // slate-900 equivalent
      borderRadius={config.thumbnail.borderRadius} // Apply border radius to the main container
      overflow="hidden"
      alignItems={horizontal ? "center" : "stretch"} // Align items differently based on layout
      gap={horizontal ? config.gapHorizontal : config.gapVertical}
      hoverStyle={{
        backgroundColor: "$purple6Dark",
      }}
    >
      {/* Thumbnail Section */}
      <Stack
        width={config.thumbnail.width}
        height={config.thumbnail.height}
        borderRadius={config.thumbnail.borderRadius}
        overflow="hidden"
        position="relative" // Needed for absolute positioning of LIVE pill
        alignSelf={horizontal ? "auto" : "center"} // Center thumbnail in vertical layout if needed
      >
        <Image
          source={{ uri: thumbnailUrl }}
          style={{ width: "100%", height: "100%" }}
          resizeMode="cover"
        />
        {isLive && (
          <XStack
            position="absolute"
            top={config.contentPadding / 2} // Adjust position based on padding
            right={config.contentPadding / 2}
            backgroundColor="$background075" // A red color
            borderRadius={999} // Pill shape
            borderWidth={"$1"}
            borderColor="#7774"
            paddingHorizontal={config.livePill.paddingHorizontal}
            height={config.livePill.height}
            alignItems="center"
            justifyContent="center"
            gap={config.livePill.gap}
            shadowColor="$background075"
            shadowOffset={{ width: 0, height: 2 }}
            shadowOpacity={0.25}
            shadowRadius={4}
          >
            <Viewers viewers={viewers} />
          </XStack>
        )}
      </Stack>

      {/* Content Section (Avatar + Text Block) */}
      <XStack
        padding={config.contentPadding}
        alignItems="center"
        gap={config.contentPadding}
        width={horizontal ? undefined : "100%"}
      >
        {/* Avatar */}
        <Stack
          width={config.avatarSize}
          height={config.avatarSize}
          borderRadius="50%"
          overflow="hidden"
        >
          <Image
            source={{ uri: avatarUrl }}
            style={{ width: "100%", height: "100%" }}
            resizeMode="cover"
          />
        </Stack>

        {/* Text Content (Title + Streamer + Category) */}
        <YStack
          justifyContent="space-between"
          alignItems="flex-start"
          gap={config.contentPadding / 2}
        >
          {title && (
            <Text
              fontSize={config.titleFontSize}
              color="white"
              fontWeight="400"
              numberOfLines={1}
              ellipsizeMode="tail"
            >
              {title}
            </Text>
          )}
          {streamerName && (
            <Text
              fontSize={config.streamerFontSize}
              color="$color" // Light purple-ish color from CSS
              fontWeight="400"
              numberOfLines={1}
              ellipsizeMode="tail"
            >
              {streamerName}
            </Text>
          )}
          {category.length > 0 && (
            <XStack>
              {category.map((cat, index) => (
                <Stack
                  backgroundColor="$background075" // Same dark background as card
                  borderRadius={999} // Pill shape
                  paddingHorizontal={config.categoryPill.paddingHorizontal}
                  height={config.categoryPill.height}
                  alignSelf="flex-start" // Align pill to the start of the YStack
                  justifyContent="center"
                >
                  <Text
                    fontSize={config.categoryFontSize}
                    color="$gray5" // Gray color from CSS
                    fontWeight="400"
                  >
                    {category}
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
