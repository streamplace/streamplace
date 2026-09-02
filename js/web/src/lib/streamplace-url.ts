// Resolution order for the Streamplace server URL:
//   1. Slice storage key (set by the streamplaceSlice.setURL action).
//   2. VITE_STREAMPLACE_URL env at build time.
//   3. window.location.origin (when running in a browser).
//
// Throws if no URL is resolvable; the streamplaceSlice handles the
// empty-string case via its try/catch in the slice initializer.
//
// This module is intentionally tiny and synchronous. Anything that
// wants the URL to be reactive (auto-update without a reload) should
// read from the slice via useStreamplaceUrl() instead.

const ENV_KEY = "VITE_STREAMPLACE_URL";
const SLICE_URL_KEY = "streamplaceUrl";

export function getStreamplaceUrl(): string {
  if (typeof localStorage !== "undefined") {
    const sliceOverride = localStorage.getItem(SLICE_URL_KEY);
    if (typeof sliceOverride === "string" && sliceOverride.trim().length > 0) {
      return sliceOverride.trim().replace(/\/+$/, "");
    }
  }

  const fromEnv = import.meta.env[ENV_KEY];
  if (typeof fromEnv === "string" && fromEnv.length > 0) {
    return fromEnv.replace(/\/+$/, "");
  }

  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin.replace(/\/+$/, "");
  }

  throw new Error(
    "Could not determine streamplaceUrl. Set VITE_STREAMPLACE_URL or run the web app from a browser.",
  );
}
