// Helpers for game activity. Mirrors js/app/utils/game.ts.

interface GameMediaItem {
  mediaType: string;
  blob?: { ref?: { toString(): string } };
}

const COVER_MEDIA_TYPES = new Set(["cover", "coverSquare"]);

export function getGameCoverUrl(
  media: GameMediaItem[] | undefined,
  did: string,
): string | undefined {
  const coverItem =
    media?.find((m) => COVER_MEDIA_TYPES.has(m.mediaType)) ?? media?.[0];
  const cid = coverItem?.blob?.ref?.toString();
  if (!cid) return undefined;
  return `https://cdn.bsky.app/img/feed_thumbnail/plain/${did}/${cid}@jpeg`;
}

export function getDidFromAtUri(uri: string): string {
  return uri.split("/")[2] ?? "";
}
