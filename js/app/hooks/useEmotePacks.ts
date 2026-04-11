import { usePossiblyUnauthedPDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import type { EmojiPack } from "components/emoji-picker/emoji-picker";
import { useEffect, useState } from "react";

export function useEmotePacks(streamerDID: string | null): EmojiPack[] {
  const agent = usePossiblyUnauthedPDSAgent();
  const [packs, setPacks] = useState<EmojiPack[]>([]);

  useEffect(() => {
    if (!agent || !streamerDID) return;
    let cancelled = false;

    agent.place.stream.emote
      .getEmotePacks({ streamer: streamerDID })
      .then((res) => {
        if (cancelled) return;
        const emojiPacks: EmojiPack[] = res.data.packs.map((pack) => ({
          name: pack.name,
          emoji: pack.emotes
            .filter((e) => e.imageUrl)
            .map((e) => ({
              name: e.name,
              imageUrl: e.imageUrl,
              aturi: e.uri,
              cid: e.cid,
              alt: e.alt ?? undefined,
            })),
        }));
        setPacks(emojiPacks);
      })
      .catch((err) => {
        if (cancelled) return;
        console.error("Failed to load emote packs", err);
      });

    return () => {
      cancelled = true;
    };
  }, [agent, streamerDID]);

  return packs;
}
