import {
  Button,
  PlayerUI,
  View,
  useDanmuEnabled,
  useDanmuUnlocked,
  usePlayerStore,
  useSetDanmuEnabled,
  useTheme,
  zero,
} from "@streamplace/components";
import {
  ChevronLeft,
  ChevronRight,
  Fullscreen,
  Minimize,
  PictureInPicture2,
} from "lucide-react-native";
import { Platform, Pressable } from "react-native";
import { VolumeSlider } from "./volume-slider";

import { Mu } from "./mu";

const { gap, layout, p, r, py, px } = zero;

interface BottomControlBarProps {
  ingest: string | null;
  pipSupported: boolean;
  pipActive: boolean;
  onHandlePip: () => void;
  dropdownPortalContainer?: any;
  showChat: boolean;
  setShowChat?: (show: boolean) => void;
}

export function BottomControlBar({
  ingest,
  pipSupported,
  pipActive,
  onHandlePip,
  dropdownPortalContainer,
  showChat,
  setShowChat,
}: BottomControlBarProps) {
  let { theme } = useTheme();
  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const setFullscreen = usePlayerStore((state) => state.setFullscreen);
  const danmuUnlocked = useDanmuUnlocked();
  const danmuEnabled = useDanmuEnabled();
  const setDanmuEnabled = useSetDanmuEnabled();

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
              <PictureInPicture2 color={theme.colors.text} />
            </View>
          </Pressable>
        )}
        {danmuUnlocked && (
          <Pressable
            onPress={() => {
              setDanmuEnabled(!danmuEnabled);
            }}
            style={[px[0], r[1]]}
          >
            <Mu
              size={22}
              color={theme.colors.text}
              style={{ opacity: danmuEnabled ? 1 : 0.5 }}
            />
          </Pressable>
        )}
        {Platform.OS === "web" && (
          <Pressable
            onPress={() => {
              setFullscreen(!fullscreen);
            }}
            style={[p[2], r[1]]}
          >
            {fullscreen ? (
              <Minimize color={theme.colors.text} />
            ) : (
              <Fullscreen color={theme.colors.text} />
            )}
          </Pressable>
        )}
        {ingest === null && (
          <PlayerUI.ContextMenu
            dropdownPortalContainer={dropdownPortalContainer}
          />
        )}
        {/* if not web, then add the collapse chat controls here */}
        {Platform.OS !== "web" && setShowChat && (
          <Button
            variant="outline"
            size="sm"
            onPress={() => {
              setShowChat(!showChat);
            }}
          >
            {showChat ? (
              <ChevronRight color="white" size={16} />
            ) : (
              <ChevronLeft color="white" size={16} />
            )}
          </Button>
        )}
      </View>
    </View>
  );
}
