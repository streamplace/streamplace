import { create } from "zustand";
import { BaseSlice, createBaseSlice } from "./slices/baseSlice";
import { BlueskySlice, createBlueskySlice } from "./slices/blueskySlice";
import {
  ContentMetadataSlice,
  createContentMetadataSlice,
} from "./slices/contentMetadataSlice";
import { PlatformSlice, createPlatformSlice } from "./slices/platformSlice";
import { SidebarSlice, createSidebarSlice } from "./slices/sidebarSlice";
import {
  StreamplaceSlice,
  createStreamplaceSlice,
} from "./slices/streamplaceSlice";

// Combined store type
export type AppStore = BaseSlice &
  SidebarSlice &
  BlueskySlice &
  ContentMetadataSlice &
  StreamplaceSlice &
  PlatformSlice;

// Create the combined store
export const useStore = create<AppStore>()((...a) => ({
  ...createBaseSlice(...a),
  ...createSidebarSlice(...a),
  ...createBlueskySlice(...a),
  ...createContentMetadataSlice(...a),
  ...createStreamplaceSlice(...a),
  ...createPlatformSlice(...a),
}));

// Export everything from slices for convenience
export * from "./slices/baseSlice";
export * from "./slices/blueskySlice";
export * from "./slices/contentMetadataSlice";
export * from "./slices/platformSlice";
export * from "./slices/sidebarSlice";
export * from "./slices/streamplaceSlice";
