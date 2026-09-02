import { Image } from "expo-image";
import { forwardRef } from "react";
import { Text, View, type ViewProps } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { borderRadius } from "../../lib/theme/tokens";

const AVATAR_SIZES = {
  xs: 20,
  sm: 24,
  md: 32,
  lg: 40,
  xl: 48,
  "2xl": 64,
} as const;

export interface AvatarProps extends ViewProps {
  /** Image URI. Falls back to the handle's first character. */
  src?: string | null;
  /** Accessible name; also the source of the fallback character. */
  name?: string;
  size?: keyof typeof AVATAR_SIZES | number;
  /**
   * Live ring: 2px live-red ring with a 2px gap. Reserved for accounts
   * that are actually live right now.
   */
  live?: boolean;
}

/** Round avatar with an optional live ring. */
export const Avatar = forwardRef<View, AvatarProps>(
  ({ src, name, size = "md", live = false, style, ...props }, ref) => {
    const { theme } = useTheme();
    const side = typeof size === "number" ? size : AVATAR_SIZES[size];
    // ring geometry: 2px ring + 2px gap on every side
    const ringInset = live ? 4 : 0;
    const outerSide = side + ringInset * 2;

    return (
      <View
        ref={ref}
        accessibilityLabel={name}
        style={[
          {
            width: outerSide,
            height: outerSide,
            borderRadius: borderRadius.full,
            alignItems: "center",
            justifyContent: "center",
            ...(live && {
              borderWidth: 2,
              borderColor: theme.colors.live,
            }),
          },
          style,
        ]}
        {...props}
      >
        <View
          style={{
            width: side,
            height: side,
            borderRadius: borderRadius.full,
            overflow: "hidden",
            backgroundColor: theme.colors.surface3,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          {src ? (
            <Image
              source={{ uri: src }}
              style={{ width: "100%", height: "100%" }}
              contentFit="cover"
              transition={100}
              accessibilityLabel={name ?? "Avatar"}
            />
          ) : (
            <Text
              style={{
                color: theme.colors.text2,
                fontSize: Math.round(side * 0.45),
              }}
            >
              {(name?.trim()?.[0] ?? "?").toUpperCase()}
            </Text>
          )}
        </View>
      </View>
    );
  },
);

Avatar.displayName = "Avatar";
