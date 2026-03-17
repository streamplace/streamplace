import { Gauge, Pause, Play } from "lucide-react-native";
import { Pressable } from "react-native";
import { useLivestreamStore } from "../../../livestream-store";
import { PlayerStatus, usePlayerStore } from "../../../player-store";

import {
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
  Text,
  useTheme,
  View,
} from "../../ui";

export function VodControls() {
  const mode = usePlayerStore((x) => x.mode);
  const status = usePlayerStore((x) => x.status);
  const togglePlayPause = usePlayerStore((x) => x.togglePlayPause);
  const quality = usePlayerStore((x) => x.selectedRendition);
  const setQuality = usePlayerStore((x) => x.setSelectedRendition);
  const liveRenditions = useLivestreamStore((x) => x.renditions);
  const vodLevels = usePlayerStore((x) => x.vodLevels);
  const renditions = mode === "vod" ? vodLevels : liveRenditions;
  const th = useTheme();

  if (mode !== "vod") return null;

  const isPlaying = status === PlayerStatus.PLAYING;

  return (
    <View
      style={{
        flexDirection: "row",
        alignItems: "center",
        paddingHorizontal: 16,
        paddingVertical: 4,
        gap: 12,
      }}
    >
      <Pressable onPress={togglePlayPause}>
        {isPlaying ? (
          <Pause
            size={22}
            color={th.theme.colors.foreground}
            fill={th.theme.colors.foreground}
          />
        ) : (
          <Play
            size={22}
            color={th.theme.colors.foreground}
            fill={th.theme.colors.foreground}
          />
        )}
      </Pressable>

      <View style={{ flex: 1 }} />

      {renditions.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger>
            <Gauge size={20} color={th.theme.colors.foreground} />
          </DropdownMenuTrigger>
          <ResponsiveDropdownMenuContent side="top" align="end">
            <DropdownMenuGroup title="Quality">
              <DropdownMenuRadioGroup
                value={quality}
                onValueChange={setQuality}
              >
                <DropdownMenuRadioItem value="source">
                  <Text>{mode === "vod" ? "Auto" : "Source"}</Text>
                </DropdownMenuRadioItem>
                {renditions.map((r) => (
                  <DropdownMenuRadioItem key={r.name} value={r.name}>
                    <Text>{r.name === "audio" ? "Audio Only" : r.name}</Text>
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuGroup>
          </ResponsiveDropdownMenuContent>
        </DropdownMenu>
      )}
    </View>
  );
}
