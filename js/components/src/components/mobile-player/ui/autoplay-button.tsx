import { Play } from "lucide-react-native";
import { Pressable } from "react-native";
import { View, layout, usePlayerStore, useSetMuted } from "../../..";
import { borderAlphas, colors, textAlphas } from "../../../lib/theme/tokens";
import { h, p, w } from "../../../ui";

export function AutoplayButton() {
  const autoplayFailed = usePlayerStore((x) => x.autoplayFailed);
  const setAutoplayFailed = usePlayerStore((x) => x.setAutoplayFailed);
  const setMuted = useSetMuted();
  const setMuteWasForced = usePlayerStore((x) => x.setMuteWasForced);
  const setUserInteraction = usePlayerStore((x) => x.setUserInteraction);
  const videoRef = usePlayerStore((x) => x.videoRef);

  const handlePlayButtonPress = () => {
    if (videoRef && typeof videoRef === "object" && videoRef.current) {
      videoRef.current
        .play()
        .then(() => {
          setAutoplayFailed(false);
          setMuted(false);
          setUserInteraction();
        })
        .catch((err) => {
          console.error("Manual play failed", err);
          if (err.name === "NotAllowedError") {
            setMuted(true);
            videoRef.current!.muted = true;
            videoRef
              .current!.play()
              .then(() => {
                setAutoplayFailed(false);
                setMuteWasForced(true);
                setUserInteraction();
              })
              .catch((err) => {
                console.error("Manual muted play also failed", err);
              });
          }
        });
    }
  };

  if (!autoplayFailed) return null;

  return (
    <View
      style={[
        layout.position.absolute,
        layout.flex.center,
        h.percent[100],
        w.percent[100],
      ]}
      pointerEvents="box-none"
    >
      <Pressable
        onPress={handlePlayButtonPress}
        style={[
          {
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 8,
          },
        ]}
      >
        <View
          style={[
            p[4],
            {
              backgroundColor: borderAlphas.dark.strong,
              borderRadius: 999,
              borderWidth: 2,
              borderColor: textAlphas.dark[3],
              boxShadow: `0 0px 4px ${colors.black}`,
              shadowColor: colors.black,
            },
          ]}
        >
          <Play size="48" color={textAlphas.dark[4]} fill={colors.white} />
        </View>
      </Pressable>
    </View>
  );
}
