// Selector hooks for the slices wired up in commit A. Additional
// selectors (bluesky, contentMetadata) land with commit B.
import { useStore } from "./index";

// Base
export const useHydrated = () => useStore((state) => state.hydrated);

// Sidebar
export const useIsSidebarCollapsed = () =>
  useStore((state) => state.isCollapsed);
export const useSidebarTargetWidth = () =>
  useStore((state) => state.targetWidth);
export const useIsSidebarLoaded = () => useStore((state) => state.isLoaded);
export const useIsSidebarHidden = () => useStore((state) => state.isHidden);

// Streamplace
export const useStreamplaceUrl = () => useStore((state) => state.url);
export const useStreamplaceInitialized = () =>
  useStore((state) => state.initialized);
export const useUserMuted = () => useStore((state) => state.userMuted);
export const useChatWarned = () => useStore((state) => state.chatWarned);
export const useMySegments = () => useStore((state) => state.mySegments);

// Platform
export const useNotificationToken = () =>
  useStore((state) => state.notificationToken);
export const useNotificationDestination = () =>
  useStore((state) => state.notificationDestination);
