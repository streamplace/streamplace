import { Volume2 } from "lucide-react-native";
import { StyleSheet } from "react-native";
import { usePlayerStore } from "../../../player-store";
import { Text, View } from "../../ui";

export function AudioOnlyOverlay() {
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const setSelectedRendition = usePlayerStore((x) => x.setSelectedRendition);

  if (selectedRendition !== "audio") {
    return null;
  }

  return (
    <View style={styles.container}>
      <View style={styles.content}>
        <Volume2 color="#fff" size={48} />
        <Text size="lg" weight="semibold" style={styles.text}>
          Audio Only mode
        </Text>
        <Text
          size="sm"
          color="muted"
          style={styles.text}
          onPress={() => setSelectedRendition("source")}
        >
          Go to Settings &gt; Quality to switch back to video.
        </Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
  },
  content: {
    alignItems: "center",
    gap: 12,
    paddingHorizontal: 24,
  },
  text: {
    textAlign: "center",
  },
});
