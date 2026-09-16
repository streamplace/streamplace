import { useStreamplaceStore } from "@streamplace/components";
import { useEffect } from "react";
import { Platform } from "react-native";

const POLL_MS = 15_000;

/**
 * Single-user node: the front door is whoever the branding key
 * defaultStreamer names. Changing that key is how an operator sends
 * every viewer to a different stream, so while the front door is showing
 * a streamer the page asks the node every 15 seconds which streamer that
 * is now and reloads itself when the answer changes. Web only, and only a
 * successful answer counts: a node hiccup must not reload everyone.
 */
export function useDefaultStreamerReload(defaultStreamer: string | undefined) {
  const url = useStreamplaceStore((s) => s.url);
  const broadcasterDID = useStreamplaceStore((s) => s.broadcasterDID);

  useEffect(() => {
    if (Platform.OS !== "web" || !defaultStreamer || !broadcasterDID) return;
    const started = defaultStreamer.trim();
    const endpoint = `${url ?? ""}/xrpc/place.stream.branding.getBlob?key=defaultStreamer&broadcaster=${encodeURIComponent(broadcasterDID)}`;
    let stopped = false;
    const check = async () => {
      try {
        const res = await fetch(endpoint, { cache: "no-store" });
        if (!res.ok) return;
        const now = (await res.text()).trim();
        if (stopped || !now || now === started) return;
        window.location.reload();
      } catch {
        // offline or a flaky node: try again next tick
      }
    };
    const timer = setInterval(check, POLL_MS);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [defaultStreamer, url, broadcasterDID]);
}
