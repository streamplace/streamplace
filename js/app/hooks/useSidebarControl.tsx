import { useState } from "react";
import {
  SharedValue,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useWindowDimensions } from "tamagui";

// Returns *true* if the screen is > 1024px
function useIsLargeScreen() {
  const { width } = useWindowDimensions();
  // gtMd breakpoint
  return width >= 980 + 1;
}

export interface UseSidebarOutput {
  isActive: boolean;
  isCollapsed: boolean;
  width: SharedValue<number>;
  toggle: () => void;
}

/*
 * useSidebarControl
 * A hook to control the custom sidebar on desktop
 *
 * Returns: An interface containing:
 * - isActive: boolean - True if the screen is considered large (width >= 1024px).
 * - sidebarWidth: Animated.Value - An animated value controlling the sidebar's width.
 * - toggleSidebar: () => void - A function to toggle the sidebar's collapsed state with animation.
 */
export default function useSidebarControl(): UseSidebarOutput {
  const [collapsed, setCollapsed] = useState(false);
  const width = useSharedValue(300);

  const isActive = useIsLargeScreen();
  const toggle = () => {
    const toValue = collapsed ? 250 : 64;
    console.log("Setting off changing to", toValue);
    width.value = withTiming(toValue, { duration: 200 });
    setCollapsed(!collapsed);
  };

  return {
    isActive,
    isCollapsed: collapsed,
    width,
    toggle,
  };
}
