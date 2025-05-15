import { createAppSlice } from "../../hooks/createSlice";
import WebStorage from "../../storage/storage";

const storage = new WebStorage();
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
          return JSON.parse(storedStateString) as SidebarState;
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
