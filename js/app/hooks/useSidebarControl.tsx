import { useEffect } from "react";
import { useWindowDimensions } from "react-native";
import {
  SharedValue,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useStore } from "store";
import {
  useIsSidebarCollapsed,
  useIsSidebarHidden,
  useSidebarTargetWidth,
} from "store/hooks";

// Returns *true* if the screen is > 1024px
function useIsLargeScreen() {
  const { width } = useWindowDimensions();
  // gtMd breakpoint
  return width >= 980 + 1;
}

export interface UseSidebarOutput {
  isActive: boolean;
  isCollapsed: boolean;
  isHidden: boolean;
  animatedWidth: SharedValue<number>;
  toggle: () => void;
}

/*
 * useSidebarControl
 * A hook to control the custom sidebar on desktop, using Redux for state.
 *
 * Returns: An interface containing:
 * - isActive: boolean - True if the screen is considered large (width >= 1024px).
 * - isCollapsed: boolean - The current collapsed state of the sidebar from Redux.
 * - animatedWidth: SharedValue<number> - An animated value controlling the sidebar's width.
 * - toggle: () => void - A function to dispatch the Redux action to toggle the sidebar.
 */
export function useSidebarControl(): UseSidebarOutput {
  const toggleSidebar = useStore((state) => state.toggleSidebar);
  const isCollapsed = useIsSidebarCollapsed();
  const targetWidth = useSidebarTargetWidth();

  const isHidden = useIsSidebarHidden();

  const animatedWidth = useSharedValue(targetWidth);

  const isActive = useIsLargeScreen();
  useEffect(() => {
    if (isActive) {
      if (!isHidden && targetWidth < 64) targetWidth == 64;
      // Only animate if the sidebar is active
      animatedWidth.value = withTiming(targetWidth, { duration: 250 });
    } else {
      animatedWidth.value = targetWidth;
    }
  }, [targetWidth, isActive, animatedWidth]);

  const handleToggle = () => {
    if (isActive) {
      // Only allow toggle if the sidebar functionality is active
      toggleSidebar();
    }
  };

  return {
    isActive,
    isCollapsed,
    animatedWidth,
    isHidden,
    toggle: handleToggle,
  };
}
