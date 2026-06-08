// Sidebar control hook. The web's sidebar is a shadcn primitive, so
// the reanimated animatedWidth from the app doesn't apply — the
// component handles its own CSS transition. We still expose
// isActive / isCollapsed / isHidden / targetWidth / toggle for
// parity and so future sidebar-overlay work has a single source of
// truth.
import { useStore } from "../lib/store";
import {
  useIsSidebarCollapsed,
  useIsSidebarHidden,
  useSidebarTargetWidth,
} from "../lib/store/hooks";
import { useIsLargeScreen } from "./use-is-large-screen";

export interface UseSidebarOutput {
  isActive: boolean;
  isCollapsed: boolean;
  isHidden: boolean;
  targetWidth: number;
  toggle: () => void;
}

export function useSidebarControl(): UseSidebarOutput {
  const isActive = useIsLargeScreen();
  const isCollapsed = useIsSidebarCollapsed();
  const isHidden = useIsSidebarHidden();
  const targetWidth = useSidebarTargetWidth();
  const toggleSidebar = useStore((state) => state.toggleSidebar);

  return {
    isActive,
    isCollapsed,
    isHidden,
    targetWidth,
    toggle: () => {
      if (isActive) toggleSidebar();
    },
  };
}
