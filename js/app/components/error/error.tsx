import { Button, Text, zero } from "@streamplace/components";
import { View } from "react-native";

export default function (props: { onRetry: () => void }) {
  return (
    <View
      style={[
        zero.flex.values[1],
        { justifyContent: "center", alignItems: "center" },
      ]}
    >
      <Text>Unable to contact server.</Text>
      <Button onPress={props.onRetry} width="min" style={zero.mt[2]}>
        Retry
      </Button>
    </View>
  );
}
