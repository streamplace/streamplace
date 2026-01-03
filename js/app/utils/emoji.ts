// emoji cache file

import { EmojiData } from "@streamplace/components/src/components/chat/emoji-suggestions";

let emoji: EmojiData | null = null;

export function loadEmojiData(): EmojiData | null {
  if (emoji) {
    return emoji;
  }
  try {
    // dynamically import the emoji data
    emoji = require("../assets/emoji-data.json");
    return emoji;
  } catch (e) {
    console.warn("Failed to load emoji data:", e);
    return null;
  }
}
