import { Image } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";

export const BADGE_IMAGES: Record<string, ReturnType<typeof require>> = {
  "place.stream.badge.defs#mod": require("../../../assets/badges/mod.png"),
  "place.stream.badge.defs#streamer": require("../../../assets/badges/live.png"),
  "place.stream.badge.defs#vip": require("../../../assets/badges/vip.png"),
};

export const Badge = ({
  badgeType,
  size = 18,
}: {
  badgeType: string;
  size?: number;
}) => {
  const source =
    BADGE_IMAGES[badgeType] ?? BADGE_IMAGES["place.stream.badge.defs#vip"];
  return (
    <Image
      source={source}
      style={{
        height: size,
        width: size,
        marginBottom: -size / 5,
        marginRight: 2,
      }}
    />
  );
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
        <Badge key={`badge-${index}`} badgeType={badge.badgeType} />
      ))}
    </>
  );
};
