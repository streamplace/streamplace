import { createListenerMiddleware, isAnyOf } from "@reduxjs/toolkit";
import { storage } from "@streamplace/components";
import { SIDEBAR_STORAGE_KEY, sidebarSlice } from "features/base/sidebarSlice";
import { RootState } from "./store";

export const listenerMiddleware = createListenerMiddleware();

listenerMiddleware.startListening({
  matcher: isAnyOf(sidebarSlice.actions.toggleSidebar),
  effect: async (action, listenerApi) => {
    const state = listenerApi.getState() as RootState;
    const sidebarStateToSave = state.sidebar;

    try {
      await storage.setItem(
        SIDEBAR_STORAGE_KEY,
        JSON.stringify(sidebarStateToSave),
      );
      console.log("Sidebar state saved to localStorage.");
    } catch (error) {
      console.error("Failed to save sidebar state to storage:", error);
    }
  },
});
