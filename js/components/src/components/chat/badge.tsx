import { Image } from "expo-image";
import { Platform } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";

export const BADGE_IMAGES: Record<string, ReturnType<typeof require>> = {
  "place.stream.badge.defs#mod": require("../../../assets/badges/mod_2x.png"),
  "place.stream.badge.defs#streamer": require("../../../assets/badges/live_2x.png"),
  "place.stream.badge.defs#vip": require("../../../assets/badges/vip_2x.png"),
  "place.stream.badge.defs#bot": require("../../../assets/badges/robot_2x.png"),
};

export const Badge = ({
  badgeType,
  size = 16,
  isInChat = false,
  imageUrl,
}: {
  badgeType: string;
  size?: number;
  isInChat?: boolean;
  imageUrl?: string;
}) => {
  const source = BADGE_IMAGES[badgeType];
  if (source) {
    return (
      <Image
        source={source}
        style={[
          {
            height: size,
            width: size,
            marginRight: 2,
          },
          isInChat && {
            marginBottom: Platform.OS === "web" ? -size : 0,
            transform:
              Platform.OS === "web" ? [{ translateY: -size / 1.3 }] : [],
          },
        ]}
      />
    );
  }
  if (imageUrl) {
    return (
      <Image
        source={{ uri: imageUrl }}
        style={[
          {
            height: size,
            width: size,
            marginRight: 2,
            borderRadius: 2,
          },
          isInChat && {
            marginBottom: Platform.OS === "web" ? -size : 0,
            transform:
              Platform.OS === "web" ? [{ translateY: -size / 1.3 }] : [],
          },
        ]}
      />
    );
  }
  return null;
};

export const BadgeDisplayRow = ({
  badges,
}: {
  badges: ChatMessageViewHydrated["badges"];
}) => {
  if (!badges?.length) return null;
  return (
    <>
      {badges.map((badge, index) => (
        <Badge
          key={index}
          badgeType={badge.badgeType}
          imageUrl={badge.imageUrl}
          isInChat
        />
      ))}
    </>
  );
};
