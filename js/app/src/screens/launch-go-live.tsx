import { useNavigation } from "@react-navigation/native";
import { Button, Text, useTheme, View, zero } from "@streamplace/components";
import { RadioTower, Video } from "lucide-react-native";

export default function LaunchGoLive() {
  const navigation = useNavigation();
  const theme = useTheme();

  return (
    <View
      style={[
        zero.layout.flex.center,
        zero.h.percent[100],
        zero.gap.all[6],
        zero.px[4],
      ]}
    >
      <View
        style={[
          zero.layout.flex.column,
          zero.gap.all[4],
          zero.layout.flex.alignCenter,
        ]}
      >
        <View style={[zero.r.full]}>
          <Video size={48} color={theme.theme.colors.text1} />
        </View>
        <Text size="2xl" weight="semibold" center>
          Ready to go live?
        </Text>
      </View>
      <View style={[zero.layout.flex.center]}>
        <Button
          size="lg"
          width="min"
          onPress={() => {
            navigation.navigate("MobileGoLive");
          }}
          leftIcon={<RadioTower color={theme.theme.colors.primaryForeground} />}
        >
          Start streaming
        </Button>
      </View>
    </View>
  );
}
