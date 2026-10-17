// Helpers for game activity. Platform-agnostic.
import { games } from "streamplace";

const COVER_MEDIA_TYPES: Set<
  games.gamesgamesgamesgames.defs.MediaItem["mediaType"]
> = new Set(["cover", "coverSquare"]);

export function getGameCoverUrl(
  media: games.gamesgamesgamesgames.defs.MediaItem[] | undefined,
  did: string,
): string | undefined {
  const coverItem =
    media?.find((m) => COVER_MEDIA_TYPES.has(m.mediaType)) ?? media?.[0];
  const cid = (coverItem?.blob as any)?.ref?.toString();
  if (!cid) return undefined;
  return `https://cdn.bsky.app/img/feed_thumbnail/plain/${did}/${cid}@jpeg`;
}

export function getDidFromAtUri(uri: string): string {
  return uri.split("/")[2] ?? "";
}
