import {
  Button,
  IconButton,
  PlayerUI,
  View,
  useDanmuEnabled,
  useDanmuUnlocked,
  usePlayerStore,
  useSetDanmuEnabled,
  zero,
} from "@streamplace/components";
import {
  colors,
  spacing,
  touchTargets,
} from "@streamplace/components/src/lib/theme/tokens";
import {
  ChevronLeft,
  ChevronRight,
  Fullscreen,
  Minimize,
  PictureInPicture2,
} from "lucide-react-native";
import type { ReactNode } from "react";
import { Platform } from "react-native";
import { useIsSidebarCollapsed } from "store/hooks";
import { Mu } from "./mu";
import { VolumeSlider } from "./volume-slider";

const { gap, layout, px } = zero;

const controlTarget = {
  width: touchTargets.minimum,
  height: touchTargets.minimum,
};

interface BottomControlBarProps {
  ingest: string | null;
  pipSupported: boolean;
  pipActive: boolean;
  onHandlePip?: () => void;
  dropdownPortalContainer?: string;
  showContextMenu?: boolean;
  onMenuOpenChange?: (open: boolean) => void;
  onScrubbingChange?: (active: boolean) => void;
  volumeSliderWidth?: number;
  liveStats?: ReactNode;
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
  if (Platform.OS !== "web") return null;
  return (
    <IconButton
      accessibilityLabel="Picture in picture"
      onPress={onHandlePip}
      disabled={pipActive}
      style={controlTarget}
    >
      <View style={{ opacity: pipActive ? 0.5 : 1 }}>
        <PictureInPicture2 size={spacing[6]} color={colors.white} />
      </View>
    </IconButton>
  );
}

function DanmuButton() {
  const danmuUnlocked = useDanmuUnlocked();
  const danmuEnabled = useDanmuEnabled();
  const setDanmuEnabled = useSetDanmuEnabled();
  if (!danmuUnlocked) return null;
  return (
    <IconButton
      accessibilityLabel={danmuEnabled ? "Disable danmu" : "Enable danmu"}
      onPress={() => setDanmuEnabled(!danmuEnabled)}
      style={controlTarget}
    >
      <Mu
        size={spacing[6]}
        color={colors.white}
        style={{ opacity: danmuEnabled ? 1 : 0.5 }}
      />
    </IconButton>
  );
}

function ContextMenuButton({
  dropdownPortalContainer,
  onMenuOpenChange,
}: {
  dropdownPortalContainer?: any;
  onMenuOpenChange?: (open: boolean) => void;
}) {
  return (
    <PlayerUI.ContextMenu
      dropdownPortalContainer={dropdownPortalContainer}
      onOpenChange={onMenuOpenChange}
    />
  );
}

function FullscreenButton() {
  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const setFullscreen = usePlayerStore((state) => state.setFullscreen);
  if (Platform.OS !== "web") return null;
  return (
    <IconButton
      accessibilityLabel={fullscreen ? "Exit fullscreen" : "Enter fullscreen"}
      onPress={() => setFullscreen(!fullscreen)}
      style={controlTarget}
    >
      {fullscreen ? (
        <Minimize size={spacing[6]} color={colors.white} />
      ) : (
        <Fullscreen size={spacing[6]} color={colors.white} />
      )}
    </IconButton>
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
    <Button
      variant="secondary"
      size="sm"
      onPress={() => setShowChat(!showChat)}
    >
      {showChat ? (
        <ChevronRight color={colors.white} size={spacing[4]} />
      ) : (
        <ChevronLeft color={colors.white} size={spacing[4]} />
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
  showContextMenu = true,
  onMenuOpenChange,
  onScrubbingChange,
  volumeSliderWidth = 200,
  liveStats,
  showChat,
  setShowChat,
}: BottomControlBarProps) {
  const sidebarCollapsed = useIsSidebarCollapsed();
  const playbackMode = usePlayerStore((x) => x.mode);
  const volume = (
    <VolumeSlider
      key={String(sidebarCollapsed)}
      sliderWidth={volumeSliderWidth}
    />
  );

  return (
    <View>
      <PlayerUI.SeekBar onScrubbingChange={onScrubbingChange} />
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          px[2],
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter]}>
          {playbackMode === "vod" ? (
            <PlayerUI.VodControls>{volume}</PlayerUI.VodControls>
          ) : (
            (liveStats ?? volume)
          )}
        </View>

        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[1]]}>
          {liveStats && volume}
          {pipSupported && onHandlePip && (
            <PipButton pipActive={pipActive} onHandlePip={onHandlePip} />
          )}
          <DanmuButton />
          {ingest === null && showContextMenu && (
            <ContextMenuButton
              dropdownPortalContainer={dropdownPortalContainer}
              onMenuOpenChange={onMenuOpenChange}
            />
          )}
          <FullscreenButton />
          {setShowChat && (
            <CollapseChatButton showChat={showChat} setShowChat={setShowChat} />
          )}
        </View>
      </View>
    </View>
  );
}
