// Sidebar visibility/collapse state, persisted to storage.
// Mirrors js/app/store/slices/sidebarSlice.ts. The web's current Sidebar
// is a shadcn primitive with its own localStorage key ("streamplace:nav-open")
// — the values here are kept in this slice for parity with the app so that
// future port work (sidebar overlay, etc.) has a single source of truth.
import { StateCreator } from "zustand";
import { storage } from "../../storage";

export const SIDEBAR_STORAGE_KEY = "sidebarState";

export interface SidebarSlice {
  isCollapsed: boolean;
  isHidden: boolean;
  targetWidth: number;
  isLoaded: boolean;
  setSidebarHidden: () => void;
  setSidebarUnhidden: () => void;
  toggleSidebar: () => void;
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
        set({ ...verifiedState, isLoaded: true });
      } else {
        set({ isLoaded: true });
      }
    } catch {
      set({
        isCollapsed: false,
        targetWidth: 250,
        isLoaded: true,
      });
    }
  },
});
