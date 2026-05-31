import { useEffect } from "react";
import { View } from "react-native";
import { PlayerProvider } from "../../player-store/player-provider";
import { usePlayerStore } from "../../player-store/player-store";
import Video from "../mobile-player/video";

// A deliberately minimal VOD player: a PlayerProvider, the src pushed into the
// player store in "vod" mode, and the platform <Video> element. No Fullscreen
// chrome, danmu, retry, telemetry, or custom seek UI — on native, expo-video's
// own controls handle scrubbing/play/pause, which is what we want for a simple
// wrapper. Contrast with <Player>, the unified live+vod player that layers all
// of that on top.
export function VodPlayer({
  src,
  objectFit = "contain",
}: {
  // at:// URI of a place.stream.video record (or any src the <Video> element
  // understands).
  src: string;
  objectFit?: "contain" | "cover";
}) {
  return (
    <PlayerProvider>
      <VodPlayerInner src={src} objectFit={objectFit} />
    </PlayerProvider>
  );
}

function VodPlayerInner({
  src,
  objectFit,
}: {
  src: string;
  objectFit: "contain" | "cover";
}) {
  const setSrc = usePlayerStore((x) => x.setSrc);
  const setMode = usePlayerStore((x) => x.setMode);
  const storeSrc = usePlayerStore((x) => x.src);

  useEffect(() => {
    setMode("vod");
    setSrc(src);
  }, [src, setMode, setSrc]);

  return (
    <View style={{ width: "100%", height: "100%", backgroundColor: "#000" }}>
      {/* Wait until our src is in the store so <Video> never renders against a
          stale/empty source (which would resolve to a bogus live playlist). */}
      {storeSrc === src ? <Video objectFit={objectFit} /> : null}
    </View>
  );
}
