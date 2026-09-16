import { useStreamplaceStore } from "@streamplace/components";
import { useEffect } from "react";
import { Platform } from "react-native";

const POLL_MS = 15_000;
const KEYS = ["defaultStreamer", "defaultVideo"] as const;

/**
 * The node's front door follows two branding keys: defaultStreamer (a
 * stream page) and defaultVideo (a video page, which wins while set).
 * Changing either is how an operator sends every viewer somewhere else, so
 * while the front page is showing one of them it asks the node every 15
 * seconds what both keys say now (two small text reads) and reloads itself
 * when either differs from what it started with. Web only. A 404 means the
 * key is unset (that is a change too: a cleared defaultVideo returns the
 * front door to the streamer); any other failure is ignored until the next
 * tick, so a node hiccup reloads nobody.
 */
export function useFrontDoorReload(
  defaultStreamer: string | undefined,
  defaultVideo: string | undefined,
) {
  const url = useStreamplaceStore((s) => s.url);
  const broadcasterDID = useStreamplaceStore((s) => s.broadcasterDID);

  useEffect(() => {
    if (Platform.OS !== "web" || !broadcasterDID) return;
    if (!defaultStreamer && !defaultVideo) return;
    const started: Record<string, string> = {
      defaultStreamer: (defaultStreamer ?? "").trim(),
      defaultVideo: (defaultVideo ?? "").trim(),
    };
    const endpoint = (key: string) =>
      `${url ?? ""}/xrpc/place.stream.branding.getBlob?key=${key}&broadcaster=${encodeURIComponent(broadcasterDID)}`;
    let stopped = false;
    // The key's value now: "" when unset (404), undefined when unknown.
    const read = async (key: string): Promise<string | undefined> => {
      try {
        const res = await fetch(endpoint(key), { cache: "no-store" });
        if (res.status === 404) return "";
        if (!res.ok) return undefined;
        return (await res.text()).trim();
      } catch {
        return undefined;
      }
    };
    const check = async () => {
      const now = await Promise.all(KEYS.map(read));
      if (stopped) return;
      const changed = KEYS.some(
        (key, i) => now[i] !== undefined && now[i] !== started[key],
      );
      if (changed) window.location.reload();
    };
    const timer = setInterval(check, POLL_MS);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [defaultStreamer, defaultVideo, url, broadcasterDID]);
}
