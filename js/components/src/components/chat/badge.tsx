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
  textBaseline = false,
  imageUrl,
  testID,
}: {
  badgeType: string;
  size?: number;
  /**
   * The badge sits in a line of text on the web, where the image is pulled up
   * onto that line. A flex row centers its items on its own, and the pull there
   * lifts the badge above the line instead, so the chat rows do not ask for it.
   */
  textBaseline?: boolean;
  imageUrl?: string;
  testID?: string;
}) => {
  const source = BADGE_IMAGES[badgeType];
  if (source) {
    return (
      <Image
        testID={testID}
        source={source}
        style={[
          {
            height: size,
            width: size,
            marginRight: 2,
          },
          textBaseline && {
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
        testID={testID}
        source={{ uri: imageUrl }}
        style={[
          {
            height: size,
            width: size,
            marginRight: 2,
            borderRadius: 2,
          },
          textBaseline && {
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
          testID="chat-badge"
          badgeType={badge.badgeType}
          imageUrl={badge.imageUrl}
        />
      ))}
    </>
  );
};
