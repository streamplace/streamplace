// Video utility functions. Copied from js/app/utils/video.ts —
// platform-agnostic, no React Native deps.

import { place } from "streamplace";

// The DID lives in the second path segment of an at:// URI:
// at://<did>/place.stream.video/<tid>
export function getDidFromAtUri(uri: string): string {
  return uri.split("/")[2] ?? "";
}

// The record key (tid) is the final segment of the at:// URI.
export function getTidFromAtUri(uri: string): string {
  return uri.split("/").pop() ?? "";
}

// VOD thumbnails are stored as blobs on the author's PDS and served
// through the Bluesky image CDN. Returns undefined when the video has
// no thumbnail so callers can fall back to a placeholder.
export function getVideoThumbnailUrl(
  record: place.stream.video.Main | undefined,
  did: string,
): string | undefined {
  const thumb = record?.thumb as any;
  const ref = thumb?.ref as { $link?: string } | undefined;
  const cid = ref?.$link ?? thumb?.ref?.toString();
  if (!cid || !did) return undefined;
  return `https://cdn.bsky.app/img/feed_thumbnail/plain/${did}/${cid}@jpeg`;
}

// Format a millisecond duration as a compact runtime badge, e.g.
// 9:05 or 1:09:05. Returns undefined for missing/zero durations.
export function formatDuration(
  durationMs: number | undefined,
): string | undefined {
  if (!durationMs || durationMs <= 0) return undefined;
  const totalSeconds = Math.floor(durationMs / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const two = (n: number) => n.toString().padStart(2, "0");
  if (hours > 0) {
    return `${hours}:${two(minutes)}:${two(seconds)}`;
  }
  return `${minutes}:${two(seconds)}`;
}
