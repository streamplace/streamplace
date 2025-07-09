import { usePlayerStore, zero } from "@streamplace/components";
import { Fullscreen, Minimize } from "lucide-react-native";
import React from "react";
import { Pressable, StyleSheet, ViewStyle } from "react-native";

const { p, r } = zero;

export interface FullscreenToggleProps {
  /**
   * Properties for the icon component
   */
  iconProps?: {
    size?: number;
    color?: string;
  };
  /**
   * Custom style for the toggle button
   */
  style?: ViewStyle | ViewStyle[];
  /**
   * Target element to make fullscreen (defaults to document.documentElement)
   */
  targetRef?: React.RefObject<HTMLElement>;
  /**
   * Optional callback function when fullscreen state changes
   */
  onFullscreenChange?: (isFullscreen: boolean) => void;
  /**
   * Optional custom render function for the icon
   */
  renderIcon?: (isFullscreen: boolean) => React.ReactNode;
  /**
   * Disable the button (e.g., on platforms where fullscreen is not supported)
   */
  disabled?: boolean;
}

/**
 * A toggle button component for controlling fullscreen mode
 */
export function FullscreenToggle({
  iconProps = { size: 20, color: "white" },
  style,
  renderIcon,
  disabled = false,
}: FullscreenToggleProps) {
  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const setFullscreen = usePlayerStore((state) => state.setFullscreen);

  return (
    <Pressable
      onPress={() => {
        setFullscreen(!fullscreen);
      }}
      style={[p[2], r[1], styles.button, style]}
      disabled={disabled}
    >
      {renderIcon ? (
        renderIcon(fullscreen)
      ) : fullscreen ? (
        <Minimize size={iconProps.size} color={iconProps.color} />
      ) : (
        <Fullscreen size={iconProps.size} color={iconProps.color} />
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    justifyContent: "center",
    alignItems: "center",
  },
});

export default FullscreenToggle;
