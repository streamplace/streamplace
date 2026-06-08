// Public API of @streamplace/core.
// Only platform-agnostic code lives here. No react-native, no expo-*.
//
// During the extraction phase, some modules still pull a couple of
// "leaves" from @streamplace/components (e.g. useDID, usePDSAgent).
// Those will move here in subsequent refactors. Anything imported from
// @streamplace/components in this file is by design temporary.
export * from "./vod-store";

export * from "./livestream-store";

export * from "./lib/browser";
export * from "./lib/facet";
export { getPlatform } from "./lib/platform";
export type { CorePlatform } from "./lib/platform";
