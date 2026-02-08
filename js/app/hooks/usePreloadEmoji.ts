import { init } from "emoji-mart";
import React from "react";
import { Platform } from "react-native";

let loadRequested = false;

export function usePreloadEmoji({ immediate }: { immediate?: boolean } = {}) {
  const preload = React.useCallback(async () => {
    if (loadRequested) {
      return;
    }
    loadRequested = true;
    let data;
    if (Platform.OS === "web") {
      data = (await import("../assets/emoji-data.json")).default;
    } else {
      data = require("../assets/emoji-data.json");
    }
    init({ data });
  }, []);

  if (immediate) {
    preload();
  }
  return preload;
}
