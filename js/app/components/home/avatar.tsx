import { Text } from "@streamplace/components";
import { Image } from "expo-image";
import { View } from "react-native";

interface UserAvatarProps {
  src?: string | null;
  size?: number;
  style?: any;
}

export function UserAvatar({
  src,
  size = 32,
  style,
  ...props
}: UserAvatarProps) {
  return (
    <View
      style={[
        {
          width: size,
          height: size,
          borderRadius: size / 2,
          overflow: "hidden",
          backgroundColor: "#666",
          alignItems: "center",
          justifyContent: "center",
        },
        style,
      ]}
      {...props}
    >
      {src ? (
        <Image
          source={{ uri: src }}
          style={{ width: "100%", height: "100%" }}
          accessibilityLabel="User avatar"
          contentFit="cover"
        />
      ) : (
        <Text style={[{ fontSize: size * 0.5, color: "#fff" }]}>🐟</Text>
      )}
    </View>
  );
}

export default UserAvatar;
