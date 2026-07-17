import { useEffect, useRef } from "react";
import { useWindowDimensions } from "react-native";
import {
  SharedValue,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useStore } from "store";
import {
  useIsSidebarCollapsed,
  useIsSidebarDrawerOpen,
  useIsSidebarHidden,
  useIsSidebarOverlay,
  useSidebarTargetWidth,
} from "store/hooks";

// Returns *true* if the screen is > 1024px
export function useIsLargeScreen() {
  const { width } = useWindowDimensions();
  // gtMd breakpoint
  return width >= 980 + 1;
}

// Width of the sidebar when opened as an overlay drawer (always expanded).
export const DRAWER_WIDTH = 250;

export interface UseSidebarOutput {
  isActive: boolean;
  isCollapsed: boolean;
  isHidden: boolean;
  /** Detail view (Stream/Video): sidebar overlays instead of pushing content. */
  overlay: boolean;
  /** Whether the overlay drawer is currently shown. */
  drawerOpen: boolean;
  /** The sidebar's own rendered width. */
  animatedWidth: SharedValue<number>;
  /** Horizontal offset — used to slide the drawer in/out in overlay mode. */
  animatedTranslateX: SharedValue<number>;
  /** Left margin the main content should reserve (0 in overlay mode). */
  animatedContentMargin: SharedValue<number>;
  /** Same as above but a plain, reactive number — use where layout must react
   *  to mode changes (e.g. the player's width) rather than animate. */
  contentMargin: number;
  /** Scrim opacity behind the drawer (0 when docked or closed). */
  animatedScrim: SharedValue<number>;
  toggle: () => void;
}

const SCRIM_OPACITY = 0.55;

/*
 * useSidebarControl — controls the desktop web sidebar.
 *
 * Two modes:
 * - Docked (browse pages): the sidebar sits in the flow and pushes content;
 *   toggling collapses/expands it (250 <-> 64).
 * - Overlay (detail pages): the sidebar leaves the flow. Content is full width;
 *   toggling slides a 250px drawer in over a dimmed scrim.
 */
export function useSidebarControl(): UseSidebarOutput {
  const toggleSidebar = useStore((state) => state.toggleSidebar);
  const toggleDrawer = useStore((state) => state.toggleDrawer);
  const isCollapsed = useIsSidebarCollapsed();
  const targetWidth = useSidebarTargetWidth();
  const isHidden = useIsSidebarHidden();
  const overlay = useIsSidebarOverlay();
  const drawerOpen = useIsSidebarDrawerOpen();
  const isActive = useIsLargeScreen();

  // In overlay mode the drawer is always full-width; docked uses the stored width.
  const width = overlay ? DRAWER_WIDTH : targetWidth;

  // Initialize each value at its resting state for the current mode so a direct
  // load renders correctly on the first frame (no docked flash on detail views).
  const animatedWidth = useSharedValue(width);
  const animatedTranslateX = useSharedValue(
    overlay ? (drawerOpen ? 0 : -DRAWER_WIDTH) : 0,
  );
  const animatedContentMargin = useSharedValue(overlay ? 0 : targetWidth);
  const animatedScrim = useSharedValue(overlay && drawerOpen ? SCRIM_OPACITY : 0);

  const prevOverlay = useRef(overlay);
  useEffect(() => {
    // Snap instantly when switching modes (navigating in/out of a detail view)
    // so the player is full width immediately — no grow-in. Only animate
    // changes *within* a mode: the drawer sliding, or a docked collapse/expand.
    const modeSwitched = prevOverlay.current !== overlay;
    prevOverlay.current = overlay;
    const anim = (v: number) =>
      isActive && !modeSwitched ? withTiming(v, { duration: 250 }) : v;
    animatedWidth.value = anim(width);
    animatedTranslateX.value = anim(
      overlay ? (drawerOpen ? 0 : -DRAWER_WIDTH) : 0,
    );
    animatedContentMargin.value = anim(overlay ? 0 : targetWidth);
    animatedScrim.value = anim(overlay && drawerOpen ? SCRIM_OPACITY : 0);
  }, [isActive, overlay, drawerOpen, targetWidth, width]);

  const handleToggle = () => {
    if (!isActive) return;
    if (overlay) {
      toggleDrawer();
    } else {
      toggleSidebar();
    }
  };

  return {
    isActive,
    isCollapsed,
    isHidden,
    overlay,
    drawerOpen,
    animatedWidth,
    animatedTranslateX,
    animatedContentMargin,
    contentMargin: isActive && !overlay ? targetWidth : 0,
    animatedScrim,
    toggle: handleToggle,
  };
}
