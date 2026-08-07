import { storage } from "@streamplace/components";
import { StateCreator } from "zustand";

export const SIDEBAR_STORAGE_KEY = "sidebarState";

export interface SidebarSlice {
  isCollapsed: boolean;
  isHidden: boolean;
  targetWidth: number;
  isLoaded: boolean;
  // Overlay/drawer mode — on detail views (Stream, Video) the sidebar leaves
  // the flow and opens as a drawer over the dimmed content instead of pushing.
  overlay: boolean;
  drawerOpen: boolean;
  setSidebarHidden: () => void;
  setSidebarUnhidden: () => void;
  toggleSidebar: () => void;
  setOverlay: (overlay: boolean) => void;
  toggleDrawer: () => void;
  closeDrawer: () => void;
  loadStateFromStorage: () => Promise<void>;
}

function verifySidebarState(state: any): Partial<SidebarSlice> {
  const verifiedState: Partial<SidebarSlice> = {
    isCollapsed:
      typeof state.isCollapsed === "boolean" ? state.isCollapsed : false,
    isHidden: typeof state.isHidden === "boolean" ? state.isHidden : false,
    targetWidth:
      typeof state.targetWidth === "number" ? state.targetWidth : 250,
    isLoaded: false,
  };

  if (!verifiedState.isHidden) {
    if (verifiedState.targetWidth! < 64) {
      verifiedState.targetWidth = 64;
    } else if (verifiedState.targetWidth! > 250) {
      verifiedState.targetWidth = 250;
    }
  } else {
    verifiedState.targetWidth = 0;
  }

  return verifiedState;
}

export const createSidebarSlice: StateCreator<SidebarSlice> = (set, get) => ({
  isCollapsed: false,
  isHidden: false,
  targetWidth: 250,
  isLoaded: false,
  overlay: false,
  drawerOpen: false,
  setOverlay: (overlay: boolean) => {
    set((state) => {
      if ((state as SidebarSlice).overlay === overlay) return {};
      // Entering or leaving overlay mode always starts with the drawer closed.
      return { overlay, drawerOpen: false };
    });
  },
  toggleDrawer: () => {
    set((state) => ({ drawerOpen: !(state as SidebarSlice).drawerOpen }));
  },
  closeDrawer: () => {
    set({ drawerOpen: false });
  },
  setSidebarHidden: () => {
    set((state) => {
      const isHidden = true;
      const targetWidth =
        (state as SidebarSlice).isCollapsed || isHidden
          ? isHidden
            ? 0
            : 64
          : 250;
      return { isHidden, targetWidth };
    });
  },
  setSidebarUnhidden: () => {
    set((state) => {
      const isHidden = false;
      const targetWidth =
        (state as SidebarSlice).isCollapsed || isHidden
          ? isHidden
            ? 0
            : 64
          : 250;
      return { isHidden, targetWidth };
    });
  },
  toggleSidebar: () => {
    set((state) => {
      const sidebarState = state as SidebarSlice;
      const isCollapsed = !sidebarState.isCollapsed;
      const targetWidth =
        isCollapsed || sidebarState.isHidden
          ? sidebarState.isHidden
            ? 0
            : 64
          : 250;
      // persist to storage
      storage.setItem(
        SIDEBAR_STORAGE_KEY,
        JSON.stringify({
          isCollapsed,
          isHidden: sidebarState.isHidden,
          targetWidth,
          isLoaded: sidebarState.isLoaded,
        }),
      );
      return { isCollapsed, targetWidth };
    });
  },
  loadStateFromStorage: async () => {
    try {
      const storedStateString = await storage.getItem(SIDEBAR_STORAGE_KEY);
      if (storedStateString) {
        let state = JSON.parse(storedStateString);
        state.isHidden = false;
        const verifiedState = verifySidebarState(state);
        console.log("Sidebar state loaded from localStorage:", verifiedState);
        set({ ...verifiedState, isLoaded: true });
      } else {
        console.log("No sidebar state found in localStorage, using defaults.");
        set({ isLoaded: true });
      }
    } catch (error) {
      console.error(
        "Failed to load sidebar state from storage, using defaults:",
        error,
      );
      set({
        isCollapsed: false,
        targetWidth: 250,
        isLoaded: true,
      });
    }
  },
});
