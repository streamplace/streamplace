import {
  Text,
  View,
  useMuted,
  usePlayerStore,
  useSetMuted,
  zero,
} from "@streamplace/components";
import { VolumeX } from "lucide-react-native";
import { useEffect } from "react";
import { Pressable } from "react-native";

const { layout, h, w, p, px } = zero;

export function MuteOverlay() {
  const muteWasForced = usePlayerStore((state) => state.muteWasForced);
  const setMuted = useSetMuted();
  const isMuted = useMuted();
  const setMuteWasForced = usePlayerStore((state) => state.setMuteWasForced);

  // let's switch muteWasForced to false if the user unmutes lol
  useEffect(() => {
    if (!isMuted && muteWasForced) {
      setMuteWasForced(false);
    }
  }, [isMuted, muteWasForced, setMuteWasForced]);

  if (!muteWasForced || !isMuted) return null;

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
        onPress={() => {
          if (muteWasForced) {
            setMuted(false);
            setMuteWasForced(false);
          }
        }}
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
              backgroundColor: "rgba(50, 30, 30, 0.4)",
              borderRadius: 999,
              borderWidth: 2,
              borderColor: "rgba(255, 120, 120, 0.2)",
              boxShadow: "0 2px 4px rgba(0, 0, 0, 1)",
              shadowColor: "rgba(0, 0, 0, 1)",
            },
          ]}
        >
          <VolumeX size="48" color="rgba(255,120,120,0.8)" />
        </View>
        <View
          style={[
            px[2],
            {
              backgroundColor: "rgba(0,0,0, 0.8)",
              borderRadius: 999,
              borderWidth: 1,
              borderColor: "rgba(255, 120, 120, 0.1)",
              boxShadow: "0 2px 4px rgba(0, 0, 0, 1)",
              shadowColor: "rgba(0, 0, 0, 1)",
            },
          ]}
        >
          <Text style={{ color: "rgba(180,180,180,0.8)" }} size="base">
            Press to unmute
          </Text>
        </View>
      </Pressable>
    </View>
  );
}
