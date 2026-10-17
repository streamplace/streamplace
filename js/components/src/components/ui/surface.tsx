import { forwardRef } from "react";
import { View, type ViewProps } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { borderRadius } from "../../lib/theme/tokens";

export interface SurfaceProps extends ViewProps {
  /**
   * Elevation in the surface scale: 0 app background, 1 card, 2 popover,
   * 3 highest. Surfaces separate with hairline borders, not shadows.
   */
  level?: 0 | 1 | 2 | 3;
  /** Hairline border. Defaults to true for level >= 1. */
  bordered?: boolean;
  /** Corner radius token. Cards default to md; thumbnails/modals use lg. */
  radius?: keyof typeof borderRadius;
}

/**
 * A themed surface. `Card` (level 1, bordered, radius md) is the common
 * building block for panels, list rows, and cards.
 */
export const Surface = forwardRef<View, SurfaceProps>(
  ({ level = 1, bordered, radius = "md", style, children, ...props }, ref) => {
    const { theme } = useTheme();
    const surfaceColor = [
      theme.colors.surface0,
      theme.colors.surface1,
      theme.colors.surface2,
      theme.colors.surface3,
    ][level];
    const showBorder = bordered ?? level >= 1;

    return (
      <View
        ref={ref}
        style={[
          {
            backgroundColor: surfaceColor,
            borderRadius: borderRadius[radius],
            ...(showBorder && {
              borderWidth: 1,
              borderColor: theme.colors.borderSubtle,
            }),
          },
          style,
        ]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

Surface.displayName = "Surface";

export type CardProps = Omit<SurfaceProps, "level">;

/** A level-1 surface with a hairline border — the standard card. */
export const Card = forwardRef<View, CardProps>((props, ref) => (
  <Surface ref={ref} level={1} {...props} />
));

Card.displayName = "Card";
