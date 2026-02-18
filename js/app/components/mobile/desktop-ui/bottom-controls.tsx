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
import { Mu } from "./mu";
import { VolumeSlider } from "./volume-slider";

const { gap, layout, p, r, px } = zero;

interface BottomControlBarProps {
  ingest: string | null;
  pipSupported: boolean;
  pipActive: boolean;
  onHandlePip: () => void;
  dropdownPortalContainer?: any;
  showChat: boolean;
  setShowChat?: (show: boolean) => void;
}

function PipButton({
  pipActive,
  onHandlePip,
}: {
  pipActive: boolean;
  onHandlePip: () => void;
}) {
  const { theme } = useTheme();
  if (Platform.OS !== "web") return null;
  return (
    <Pressable onPress={onHandlePip} disabled={pipActive}>
      <View style={{ opacity: pipActive ? 0.5 : 1 }}>
        <PictureInPicture2 color={theme.colors.text} />
      </View>
    </Pressable>
  );
}

function DanmuButton() {
  const { theme } = useTheme();
  const danmuUnlocked = useDanmuUnlocked();
  const danmuEnabled = useDanmuEnabled();
  const setDanmuEnabled = useSetDanmuEnabled();
  if (!danmuUnlocked) return null;
  return (
    <Pressable
      onPress={() => setDanmuEnabled(!danmuEnabled)}
      style={[px[2], r[1]]}
    >
      <Mu
        size={22}
        color={theme.colors.text}
        style={{ opacity: danmuEnabled ? 1 : 0.5 }}
      />
    </Pressable>
  );
}

function ContextMenuButton({
  dropdownPortalContainer,
}: {
  dropdownPortalContainer?: any;
}) {
  return (
    <PlayerUI.ContextMenu dropdownPortalContainer={dropdownPortalContainer} />
  );
}

function FullscreenButton() {
  const { theme } = useTheme();
  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const setFullscreen = usePlayerStore((state) => state.setFullscreen);
  if (Platform.OS !== "web") return null;
  return (
    <Pressable onPress={() => setFullscreen(!fullscreen)} style={[p[2], r[1]]}>
      {fullscreen ? (
        <Minimize color={theme.colors.text} />
      ) : (
        <Fullscreen color={theme.colors.text} />
      )}
    </Pressable>
  );
}

function CollapseChatButton({
  showChat,
  setShowChat,
}: {
  showChat: boolean;
  setShowChat: (show: boolean) => void;
}) {
  if (Platform.OS === "web") return null;
  return (
    <Button variant="outline" size="sm" onPress={() => setShowChat(!showChat)}>
      {showChat ? (
        <ChevronRight color="white" size={16} />
      ) : (
        <ChevronLeft color="white" size={16} />
      )}
    </Button>
  );
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
  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        zero.px[4],
      ]}
    >
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[4]]}>
        <VolumeSlider />
      </View>

      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        {pipSupported && (
          <PipButton pipActive={pipActive} onHandlePip={onHandlePip} />
        )}
        <DanmuButton />
        {ingest === null && (
          <ContextMenuButton
            dropdownPortalContainer={dropdownPortalContainer}
          />
        )}
        <FullscreenButton />
        {setShowChat && (
          <CollapseChatButton showChat={showChat} setShowChat={setShowChat} />
        )}
      </View>
    </View>
  );
}
