// Combined Zustand store. Mirrors the shape of js/app/store/index.ts.
// Slices are spread, not namespaced; selectors read from the flat state.
// Additional slices (bluesky, contentMetadata) are added in commit B.
import { create } from "zustand";
import { BaseSlice, createBaseSlice } from "./slices/baseSlice";
import { createPlatformSlice, PlatformSlice } from "./slices/platformSlice";
import { createSidebarSlice, SidebarSlice } from "./slices/sidebarSlice";
import {
  createStreamplaceSlice,
  StreamplaceSlice,
} from "./slices/streamplaceSlice";

export type AppStore = BaseSlice &
  SidebarSlice &
  StreamplaceSlice &
  PlatformSlice;

export const useStore = create<AppStore>()((...a) => ({
  ...createBaseSlice(...a),
  ...createSidebarSlice(...a),
  ...createStreamplaceSlice(...a),
  ...createPlatformSlice(...a),
}));

export * from "./slices/baseSlice";
export * from "./slices/platformSlice";
export * from "./slices/sidebarSlice";
export * from "./slices/streamplaceSlice";
