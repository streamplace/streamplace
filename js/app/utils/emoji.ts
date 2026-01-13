// emoji cache file

import { EmojiData } from "@streamplace/components/src/components/chat/emoji-suggestions";
import { useEffect, useState } from "react";

let emojiPromise: Promise<typeof import("../assets/emoji-data.json")> | null =
  null;

export function useEmojiData(): EmojiData | null {
  const [emoji, setEmoji] = useState<EmojiData | null>(null);

  useEffect(() => {
    if (!emojiPromise) {
      emojiPromise = import("../assets/emoji-data.json");
    }
    emojiPromise.then((emoji) => {
      setEmoji(emoji);
    });
  }, []);

  return emoji;
}
