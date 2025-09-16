import { storage } from "@streamplace/components";
import { createAppSlice } from "../../hooks/createSlice";
export const SIDEBAR_STORAGE_KEY = "sidebarState";

export interface SidebarState {
  isCollapsed: boolean;
  // should only be used in fullscreen
  isHidden: boolean;
  targetWidth: number;
  isLoaded: boolean;
}

const initialState: SidebarState = {
  isCollapsed: false,
  isHidden: false,
  targetWidth: 250,
  isLoaded: false,
};

function verifySidebarState(state: any): SidebarState {
  const verifiedState: SidebarState = {
    isCollapsed:
      typeof state.isCollapsed === "boolean" ? state.isCollapsed : false,
    isHidden: typeof state.isHidden === "boolean" ? state.isHidden : false,
    targetWidth:
      typeof state.targetWidth === "number" ? state.targetWidth : 250,
    isLoaded: false,
  };

  if (!verifiedState.isHidden) {
    if (verifiedState.targetWidth < 64) {
      verifiedState.targetWidth = 64;
    }
  } else {
    verifiedState.targetWidth = 0;
  }

  return verifiedState;
}

export const sidebarSlice = createAppSlice({
  name: "sidebar",
  initialState,
  reducers: (create) => ({
    setSidebarHidden: create.reducer((state) => {
      state.isHidden = true;
      state.targetWidth =
        state.isCollapsed || state.isHidden ? (state.isHidden ? 0 : 64) : 250;
    }),
    setSidebarUnhidden: create.reducer((state) => {
      state.isHidden = false;
      state.targetWidth =
        state.isCollapsed || state.isHidden ? (state.isHidden ? 0 : 64) : 250;
    }),
    toggleSidebar: create.reducer((state) => {
      state.isCollapsed = !state.isCollapsed;
      state.targetWidth =
        state.isCollapsed || state.isHidden ? (state.isHidden ? 0 : 64) : 250;
    }),
    loadStateFromStorage: create.asyncThunk(
      async () => {
        const storedStateString = await storage.getItem(SIDEBAR_STORAGE_KEY);
        if (storedStateString) {
          let state = JSON.parse(storedStateString);
          // should never be 'true' on load, component should ALWAYS request to hide sidebar when loaded
          state.isHidden = false;
          return verifySidebarState(state) as SidebarState;
        }
        return null;
      },
      {
        pending: (state) => {
          // unlikely that this will hang for a noticeable duration
        },
        fulfilled: (state, action) => {
          if (action.payload) {
            state.isCollapsed = action.payload.isCollapsed;
            state.targetWidth = action.payload.targetWidth;
            console.log(
              "Sidebar state loaded from localStorage:",
              action.payload,
            );
          } else {
            console.log(
              "No sidebar state found in localStorage, using defaults.",
            );
          }
          state.isLoaded = true;
        },
        rejected: (state, action) => {
          state.isLoaded = true;
          console.error(
            "Failed to load sidebar state from storage, using defaults:",
            action.error,
          );
          // use defaults
          state.isCollapsed = false;
          state.targetWidth = 250;
        },
      },
    ),
  }),
  selectors: {
    selectIsSidebarCollapsed: (state) => state.isCollapsed,
    selectSidebarTargetWidth: (state) => state.targetWidth,
    selectIsSidebarLoaded: (state) => state.isLoaded,
    selectIsSidebarHidden: (state) => state.isHidden,
  },
});

export const {
  toggleSidebar,
  loadStateFromStorage,
  setSidebarHidden,
  setSidebarUnhidden,
} = sidebarSlice.actions;
export const {
  selectIsSidebarCollapsed,
  selectSidebarTargetWidth,
  selectIsSidebarLoaded,
  selectIsSidebarHidden,
} = sidebarSlice.selectors;
