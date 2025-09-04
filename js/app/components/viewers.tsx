import { Eye } from "@tamagui/lucide-icons";
import { Text, View } from "tamagui";

export default function Viewers({ viewers }: { viewers: number }) {
  return (
    <View
      justifyContent="center"
      flexDirection="row"
      alignItems="center"
      gap="$2"
      paddingHorizontal="$2"
      paddingVertical="$1"
    >
      <Eye color="#fd5050" />
      <Text
        color="#fd5050"
        textShadowColor="black"
        textShadowOffset={{ width: -1, height: 1 }}
        textShadowRadius={3}
      >
        {new Intl.NumberFormat(undefined, { notation: "compact" }).format(
          viewers,
        )}
      </Text>
    </View>
  );
}
