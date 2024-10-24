import { View, Text, Button } from "tamagui";

export default function (props: { onRetry: () => void }) {
  return (
    <View f={1} justifyContent="center" alignItems="center">
      <Text>Unable to contact server.</Text>
      <Button>Retry?</Button>
    </View>
  );
}
