import { useEffect } from "react";
import { View } from "react-native";
import { PlayerProvider } from "../../player-store/player-provider";
import { usePlayerStore } from "../../player-store/player-store";
import Video from "../mobile-player/video";

export function VodPlayer({
  src,
  objectFit = "contain",
  children,
}: {
  src: string;
  objectFit?: "contain" | "cover";
  children?: React.ReactNode;
}) {
  return (
    <PlayerProvider>
      <VodPlayerInner src={src} objectFit={objectFit}>
        {children}
      </VodPlayerInner>
    </PlayerProvider>
  );
}

function VodPlayerInner({
  src,
  objectFit,
  children,
}: {
  src: string;
  objectFit: "contain" | "cover";
  children?: React.ReactNode;
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
      {storeSrc === src ? <Video objectFit={objectFit} /> : null}
      {children}
    </View>
  );
}
