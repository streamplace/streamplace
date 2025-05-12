import React from "react";
import { init } from "emoji-mart";
import { isWeb } from "tamagui";

let loadRequested = false;

export function usePreloadEmoji({ immediate }: { immediate?: boolean } = {}) {
  const preload = React.useCallback(async () => {
    if (loadRequested) return;
    loadRequested = true;
    let data;
    if (isWeb) {
      data = (await import("../assets/emoji-data.json")).default;
    } else {
      data = require("../assets/emoji-data.json");
    }
    init({ data });
  }, []);

  if (immediate) preload();
  return preload;
}
