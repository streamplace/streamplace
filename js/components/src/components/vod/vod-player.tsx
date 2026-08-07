import { useEffect } from "react";
import { View } from "react-native";
import { colors as tokensColors } from "../../lib/theme/tokens";
import { PlayerProvider } from "../../player-store/player-provider";
import { usePlayerStore } from "../../player-store/player-store";
import { useMuted, useSetMuted } from "../../streamplace-store";
import Video from "../mobile-player/video";

export function VodPlayer({
  src,
  objectFit = "contain",
  embedded,
  muted: mutedProp,
  pictureInPictureEnabled,
  children,
}: {
  src: string;
  objectFit?: "contain" | "cover";
  embedded?: boolean;
  muted?: boolean;
  pictureInPictureEnabled?: boolean;
  children?: React.ReactNode;
}) {
  return (
    <PlayerProvider>
      <VodPlayerInner
        src={src}
        objectFit={objectFit}
        embedded={!!embedded}
        muted={mutedProp}
        pictureInPictureEnabled={pictureInPictureEnabled}
      >
        {children}
      </VodPlayerInner>
    </PlayerProvider>
  );
}

function VodPlayerInner({
  src,
  objectFit,
  embedded,
  muted: mutedProp,
  pictureInPictureEnabled,
  children,
}: {
  src: string;
  objectFit: "contain" | "cover";
  embedded: boolean;
  muted?: boolean;
  pictureInPictureEnabled?: boolean;
  children?: React.ReactNode;
}) {
  const setSrc = usePlayerStore((x) => x.setSrc);
  const setMode = usePlayerStore((x) => x.setMode);
  const setEmbedded = usePlayerStore((x) => x.setEmbedded);
  const storeSrc = usePlayerStore((x) => x.src);

  const setMuted = useSetMuted();
  const muted = useMuted();

  useEffect(() => {
    setMode("vod");
    setSrc(src);
  }, [src, setMode, setSrc]);

  useEffect(() => {
    setEmbedded(embedded);
  }, [embedded, setEmbedded]);

  useEffect(() => {
    if (mutedProp !== undefined) {
      let wasMuted: boolean | null = muted;
      setTimeout(() => setMuted(mutedProp), 200);
      return () => {
        if (wasMuted !== null) setMuted(wasMuted);
      };
    }
  }, [mutedProp]);

  return (
    <View
      style={{
        width: "100%",
        height: "100%",
        backgroundColor: tokensColors.black,
      }}
    >
      {storeSrc === src ? (
        <Video
          objectFit={objectFit}
          pictureInPictureEnabled={pictureInPictureEnabled}
        />
      ) : null}
      {children}
    </View>
  );
}
