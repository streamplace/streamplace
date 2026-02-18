import { Image, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { zero } from "../..";

export const BADGE_IMAGES: Record<string, ReturnType<typeof require>> = {
  "place.stream.badge.defs#mod": require("../../../assets/badges/mod.png"),
  "place.stream.badge.defs#streamer": require("../../../assets/badges/live.png"),
  "place.stream.badge.defs#vip": require("../../../assets/badges/vip.png"),
};

export const Badge = ({
  badgeType,
  size = 20,
}: {
  badgeType: string;
  size?: number;
}) => {
  const source =
    BADGE_IMAGES[badgeType] ?? BADGE_IMAGES["place.stream.badge.defs#vip"];
  return (
    <Image
      source={source}
      style={{ height: size, width: size, marginTop: 3 }}
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
    <View style={[zero.layout.flex.align.end]}>
      {badges.map((badge, index) => (
        <View style={{ height: 3 }} key={`badge-${index}`}>
          <Badge badgeType={badge.badgeType} />
        </View>
      ))}
    </View>
  );
};
