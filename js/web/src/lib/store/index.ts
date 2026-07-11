// Combined Zustand store. Slices are spread, not namespaced; selectors read from the flat state.
import { create } from "zustand";
import { BaseSlice, createBaseSlice } from "./slices/baseSlice";
import { BlueskySlice, createBlueskySlice } from "./slices/blueskySlice";
import {
  ContentMetadataSlice,
  createContentMetadataSlice,
} from "./slices/contentMetadataSlice";
import { createSidebarSlice, SidebarSlice } from "./slices/sidebarSlice";
import {
  createStreamplaceSlice,
  StreamplaceSlice,
} from "./slices/streamplaceSlice";

export type AppStore = BaseSlice &
  SidebarSlice &
  StreamplaceSlice &
  BlueskySlice &
  ContentMetadataSlice;

export const useStore = create<AppStore>()((...a) => ({
  ...createBaseSlice(...a),
  ...createSidebarSlice(...a),
  ...createStreamplaceSlice(...a),
  ...createBlueskySlice(...a),
  ...createContentMetadataSlice(...a),
}));

export * from "./slices/baseSlice";
export * from "./slices/blueskySlice";
export * from "./slices/contentMetadataSlice";
export * from "./slices/sidebarSlice";
export * from "./slices/streamplaceSlice";
