import { PlayerUI, View, usePlayerStore, zero } from "@streamplace/components";
import { Fullscreen, Minimize, PictureInPicture2 } from "lucide-react-native";
import { Platform, Pressable } from "react-native";
import { VolumeSlider } from "./volume-slider";

const { gap, layout, p, r } = zero;

interface BottomControlBarProps {
  ingest: string | null;
  pipSupported: boolean;
  pipActive: boolean;
  onHandlePip: () => void;
}

export function BottomControlBar({
  ingest,
  pipSupported,
  pipActive,
  onHandlePip,
}: BottomControlBarProps) {
  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const setFullscreen = usePlayerStore((state) => state.setFullscreen);

  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
      ]}
    >
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[4]]}>
        <VolumeSlider />
      </View>

      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        {Platform.OS === "web" && pipSupported && (
          <Pressable onPress={onHandlePip} disabled={pipActive}>
            <View style={{ opacity: pipActive ? 0.5 : 1 }}>
              <PictureInPicture2 />
            </View>
          </Pressable>
        )}
        {Platform.OS === "web" && (
          <Pressable
            onPress={() => {
              setFullscreen(!fullscreen);
            }}
            style={[p[2], r[1]]}
          >
            {fullscreen ? <Minimize /> : <Fullscreen />}
          </Pressable>
        )}
        {ingest === null && <PlayerUI.ContextMenu />}
      </View>
    </View>
  );
}
