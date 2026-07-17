import {
  Button,
  PlayerStatus,
  PlayerUI,
  Text,
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
  Pause,
  PictureInPicture2,
  Play,
} from "lucide-react-native";
import { Platform, Pressable } from "react-native";
import { useIsSidebarCollapsed } from "store/hooks";
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
    <Button variant="secondary" size="sm" onPress={() => setShowChat(!showChat)}>
      {showChat ? (
        <ChevronRight color="white" size={16} />
      ) : (
        <ChevronLeft color="white" size={16} />
      )}
    </Button>
  );
}

function formatTime(seconds: number): string {
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  if (h > 0) return `${h}:${pad(m)}:${pad(sec)}`;
  return `${m}:${pad(sec)}`;
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
  const th = useTheme();
  const sidebarCollapsed = useIsSidebarCollapsed();
  const playbackMode = usePlayerStore((x) => x.mode);
  const togglePlayPause = usePlayerStore((x) => x.togglePlayPause);

  const playTime = usePlayerStore((x) => x.playTime);
  const duration = usePlayerStore((x) => x.duration);

  const status = usePlayerStore((x) => x.status);
  const PlayPause = status === PlayerStatus.PLAYING ? Pause : Play;

  return (
    <View>
      <PlayerUI.SeekBar />
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          zero.px[4],
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[4]]}>
          {playbackMode === "vod" && (
            <Pressable onPress={togglePlayPause}>
              <PlayPause
                color={th.theme.colors.primaryForeground}
                fill={th.theme.colors.primaryForeground}
              />
            </Pressable>
          )}
          <VolumeSlider key={String(sidebarCollapsed)} />
          {playbackMode === "vod" && (
            <View style={[layout.flex.row, zero.gap.all[1]]}>
              <Text
                // @ts-expect-error web-only
                style={{
                  fontVariant: "tabular-nums",
                }}
              >
                {formatTime(playTime)}
              </Text>
              <Text>/</Text>
              <Text
                // @ts-expect-error web-only
                style={{
                  fontVariant: "tabular-nums",
                }}
              >
                {formatTime(duration)}
              </Text>
            </View>
          )}
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
    </View>
  );
}
