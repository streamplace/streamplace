import { TextTrackWatcher, watchTextTracks } from "@streamplace/core";
import { RefObject, useEffect, useRef } from "react";
import { usePlayerStore } from "../../player-store";
import { useCaptionSelection } from "./use-captions";

/**
 * Feeds a web <video>'s subtitle/caption TextTracks (hls.js or Safari's
 * native HLS) into the player store, showing the selected one through
 * CaptionOverlay. See watchTextTracks.
 */
export function useTextTrackCaptions(
  videoRef: RefObject<HTMLVideoElement | null>,
) {
  const setTracks = usePlayerStore((x) => x.setCaptionElementTracks);
  const setLines = usePlayerStore((x) => x.setCaptionElementLines);
  const { active } = useCaptionSelection();
  const activeKey = active?.elementKey ?? null;
  const activeKeyRef = useRef(activeKey);
  activeKeyRef.current = activeKey;
  const watcher = useRef<TextTrackWatcher | null>(null);

  // The element mounts after the first render; re-check on each render.
  const video = videoRef.current;
  useEffect(() => {
    if (!video) return;
    const w = watchTextTracks(video, setTracks, setLines);
    w.setActive(activeKeyRef.current);
    watcher.current = w;
    return () => {
      w.dispose();
      watcher.current = null;
      setTracks([]);
      setLines([]);
    };
  }, [video, setTracks, setLines]);

  useEffect(() => {
    watcher.current?.setActive(activeKey);
  }, [activeKey]);
}
