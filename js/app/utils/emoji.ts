// emoji cache file

import { EmojiData } from "@streamplace/components/src/components/chat/emoji-suggestions";
import { useState } from "react";

let emojiPromise: Promise<
  typeof import("../assets/emoji-data-stripped.json")
> | null = null;

export function useEmojiData(): EmojiData | null {
  const [emoji, setEmoji] = useState<EmojiData | null>(null);
  if (!emojiPromise) {
    emojiPromise = import("../assets/emoji-data-stripped.json");
  }
  emojiPromise.then((emoji) => {
    setEmoji(emoji);
  });
  return emoji;
}
