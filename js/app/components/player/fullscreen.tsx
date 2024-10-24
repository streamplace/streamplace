import { useEffect, useRef } from "react";
import { TamaguiElement, View } from "tamagui";
import { PlayerProps } from "./props";
import Video from "./video";
import Controls from "./controls";
import PlayerLoading from "./player-loading";

export default function Fullscreen(props: PlayerProps) {
  const ref = useRef<TamaguiElement>(null);

  const setFullscreen = (on: boolean) => {
    if (!ref.current) {
      return;
    }
    (async () => {
      if (on && !document.fullscreenElement) {
        try {
          const div = ref.current as HTMLDivElement;
          await div.requestFullscreen();
          props.setFullscreen(true);
        } catch (e) {
          console.error("fullscreen failed", e.message);
        }
      }
      if (!on && document.fullscreenElement) {
        try {
          await document.exitFullscreen();
          props.setFullscreen(false);
        } catch (e) {
          console.error("fullscreen exit failed", e.message);
        }
      }
    })();
  };

  useEffect(() => {
    const listener = () => {
      props.setFullscreen(!!document.fullscreenElement);
    };
    document.body.addEventListener("fullscreenchange", listener);
    return () => {
      document.body.removeEventListener("fullscreenchange", listener);
    };
  }, []);

  return (
    <View flex={1} ref={ref}>
      <PlayerLoading {...props}></PlayerLoading>
      <Controls {...props} setFullscreen={setFullscreen} />
      <Video {...props} />
    </View>
  );
}
