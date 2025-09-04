import { Play } from "lucide-react-native";
import { Pressable } from "react-native";
import { View, layout, usePlayerStore } from "../../..";
import { h, p, w } from "../../../ui";

export function AutoplayButton() {
  const autoplayFailed = usePlayerStore((x) => x.autoplayFailed);
  const setAutoplayFailed = usePlayerStore((x) => x.setAutoplayFailed);
  const setMuted = usePlayerStore((x) => x.setMuted);
  const setMuteWasForced = usePlayerStore((x) => x.setMuteWasForced);
  const setUserInteraction = usePlayerStore((x) => x.setUserInteraction);
  const videoRef = usePlayerStore((x) => x.videoRef);

  const handlePlayButtonPress = () => {
    if (videoRef && typeof videoRef === "object" && videoRef.current) {
      videoRef.current
        .play()
        .then(() => {
          setAutoplayFailed(false);
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
              backgroundColor: "rgba(200,200,255, 0.1)",
              borderRadius: 999,
              borderWidth: 2,
              borderColor: "rgba(200,200,255, 0.45)",
              boxShadow: "0 0px 4px rgba(0, 0, 0, 1)",
              shadowColor: "rgba(0, 0, 0, 1)",
            },
          ]}
        >
          <Play
            size="48"
            color="rgba(120,120,120,0.3)"
            fill="rgba(200,200,255,1)"
          />
        </View>
      </Pressable>
    </View>
  );
}
