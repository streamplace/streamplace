import {
  Text,
  View,
  useMuted,
  usePlayerStore,
  useSetMuted,
  zero,
} from "@streamplace/components";
import {
  borderAlphas,
  colors,
  scrims,
  statusColors,
  textAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
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
              backgroundColor: scrims.dark,
              borderRadius: 999,
              borderWidth: 2,
              borderColor: borderAlphas.dark.strong,
              boxShadow: `0 2px 4px ${colors.black}`,
              shadowColor: colors.black,
            },
          ]}
        >
          <VolumeX size="48" color={statusColors.dark.danger} />
        </View>
        <View
          style={[
            px[2],
            {
              backgroundColor: scrims.dark,
              borderRadius: 999,
              borderWidth: 1,
              borderColor: borderAlphas.dark.subtle,
              boxShadow: `0 2px 4px ${colors.black}`,
              shadowColor: colors.black,
            },
          ]}
        >
          <Text style={{ color: textAlphas.dark[2] }} size="base">
            Press to unmute
          </Text>
        </View>
      </Pressable>
    </View>
  );
}
