// Combined Zustand store. Slices are spread, not namespaced; selectors read from the flat state.
import { create } from "zustand";
import { BaseSlice, createBaseSlice } from "./slices/baseSlice";
import { BlueskySlice, createBlueskySlice } from "./slices/blueskySlice";
import { BrandingSlice, createBrandingSlice } from "./slices/brandingSlice";
import {
  ContentMetadataSlice,
  createContentMetadataSlice,
} from "./slices/contentMetadataSlice";
import {
  createDanmuSlice,
  DanmuSlice,
  hydrateDanmuSettings,
} from "./slices/danmuSlice";
import { createPlatformSlice, PlatformSlice } from "./slices/platformSlice";
import { createSidebarSlice, SidebarSlice } from "./slices/sidebarSlice";
import {
  createStreamplaceSlice,
  StreamplaceSlice,
} from "./slices/streamplaceSlice";

export type AppStore = BaseSlice &
  SidebarSlice &
  StreamplaceSlice &
  BlueskySlice &
  ContentMetadataSlice &
  PlatformSlice &
  DanmuSlice &
  BrandingSlice;

export const useStore = create<AppStore>()((...a) => ({
  ...createBaseSlice(...a),
  ...createSidebarSlice(...a),
  ...createStreamplaceSlice(...a),
  ...createBlueskySlice(...a),
  ...createContentMetadataSlice(...a),
  ...createPlatformSlice(...a),
  ...createDanmuSlice(...a),
  ...createBrandingSlice(...a),
}));

// Hydrate danmu settings from localStorage once the store exists.
hydrateDanmuSettings(useStore);

export * from "./slices/baseSlice";
export * from "./slices/blueskySlice";
export * from "./slices/brandingSlice";
export * from "./slices/contentMetadataSlice";
export * from "./slices/danmuSlice";
export * from "./slices/platformSlice";
export * from "./slices/sidebarSlice";
export * from "./slices/streamplaceSlice";
