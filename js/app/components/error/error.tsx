import { Button, Text, View } from "tamagui";

export default function (props: { onRetry: () => void }) {
  return (
    <View f={1} justifyContent="center" alignItems="center">
      <Text>Unable to contact server.</Text>
      <Button onPress={props.onRetry}>Retry?</Button>
    </View>
  );
}
