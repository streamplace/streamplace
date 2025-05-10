import { createAppSlice } from "../../hooks/createSlice";
import WebStorage from "../../storage/storage";

const storage = new WebStorage();
export const SIDEBAR_STORAGE_KEY = "sidebarState";

export interface SidebarState {
  isCollapsed: boolean;
  targetWidth: number;
  isLoaded: boolean;
}

const initialState: SidebarState = {
  isCollapsed: false,
  targetWidth: 250,
  isLoaded: false,
};

export const sidebarSlice = createAppSlice({
  name: "sidebar",
  initialState,
  reducers: (create) => ({
    toggleSidebar: create.reducer((state) => {
      state.isCollapsed = !state.isCollapsed;
      state.targetWidth = state.isCollapsed ? 64 : 250;
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
            "Failed to load sidebar state from storage:",
            action.error,
          );
        },
      },
    ),
  }),
  selectors: {
    selectIsSidebarCollapsed: (state) => state.isCollapsed,
    selectSidebarTargetWidth: (state) => state.targetWidth,
    selectIsSidebarLoaded: (state) => state.isLoaded,
  },
});

export const { toggleSidebar, loadStateFromStorage } = sidebarSlice.actions;
export const {
  selectIsSidebarCollapsed,
  selectSidebarTargetWidth,
  selectIsSidebarLoaded,
} = sidebarSlice.selectors;
