import { Volume2 } from "lucide-react-native";
import { Pressable } from "react-native";
import { zero } from "../../..";
import { colors } from "../../../lib/theme/tokens";
import { usePlayerStore } from "../../../player-store";
import { Text, View } from "../../ui";

export function AudioOnlyOverlay() {
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const setSelectedRendition = usePlayerStore((x) => x.setSelectedRendition);

  if (selectedRendition !== "audio") {
    return null;
  }

  // The whole overlay is the way back: tapping it returns to the source
  // rendition (the quality menu still works too).
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel="Switch back to video"
      onPress={() => setSelectedRendition("source")}
      style={[
        zero.layout.position.absolute,
        zero.position.top[0],
        zero.position.left[0],
        zero.position.right[0],
        zero.position.bottom[0],
        zero.layout.flex.center,
      ]}
    >
      <View
        style={[
          zero.layout.flex.column,
          zero.layout.flex.alignCenter,
          zero.gap.all[3],
          zero.px[6],
        ]}
      >
        <Volume2 color={colors.white} size={48} />
        <Text size="lg" weight="semibold" center>
          Audio Only mode
        </Text>
        <Text size="sm" color="muted" center>
          Tap to switch back to video.
        </Text>
      </View>
    </Pressable>
  );
}
