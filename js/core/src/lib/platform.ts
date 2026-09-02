// Platform detection helper. Pure JS, no react-native / expo deps.
//
// Replaces the `Platform.OS` API from react-native for use in
// @streamplace/core code. Use the core package's `getPlatform()` in
// platform-agnostic code; only fall back to react-native's `Platform` in
// UI code that lives in @streamplace/components.

export type CorePlatform =
  | "ios"
  | "android"
  | "web"
  | "macos"
  | "windows"
  | "linux"
  | "electron"
  | "node"
  | "unknown";

export function getPlatform(): CorePlatform {
  if (typeof navigator === "undefined" && typeof process !== "undefined") {
    if ((process as any).versions?.electron) return "electron";
    if ((process as any).versions?.node) return "node";
  }

  if (typeof navigator !== "undefined") {
    const ua = (navigator as any).userAgent ?? "";
    if (/iPhone|iPad|iPod/i.test(ua)) return "ios";
    if (/Android/i.test(ua)) return "android";
    if (/Macintosh/i.test(ua)) return "macos";
    if (/Windows/i.test(ua)) return "windows";
    if (/Linux/i.test(ua)) return "linux";
  }

  return "unknown";
}
