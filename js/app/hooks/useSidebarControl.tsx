import { useEffect } from "react";
import {
  SharedValue,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useDispatch, useSelector } from "react-redux";
import { useWindowDimensions } from "tamagui";

import {
  selectIsSidebarCollapsed,
  selectIsSidebarHidden,
  selectSidebarTargetWidth,
  toggleSidebar,
} from "../features/base/sidebarSlice";
import { RootState } from "../store/store";

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
  const dispatch = useDispatch();
  const isCollapsed = useSelector((state: RootState) =>
    selectIsSidebarCollapsed(state),
  );
  const targetWidth = useSelector((state: RootState) =>
    selectSidebarTargetWidth(state),
  );

  const isHidden = useSelector((state: RootState) =>
    selectIsSidebarHidden(state),
  );

  const animatedWidth = useSharedValue(targetWidth);

  const isActive = useIsLargeScreen();
  useEffect(() => {
    if (isActive) {
      // Only animate if the sidebar is active
      animatedWidth.value = withTiming(targetWidth, { duration: 250 });
    } else {
      animatedWidth.value = targetWidth;
    }
  }, [targetWidth, isActive, animatedWidth]);

  const handleToggle = () => {
    if (isActive) {
      // Only allow toggle if the sidebar functionality is active
      dispatch(toggleSidebar());
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
