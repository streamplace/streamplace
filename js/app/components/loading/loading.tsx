import { View, Text, Spinner } from "tamagui";

export default function () {
  return (
    <View f={1} alignItems="center" justifyContent="center">
      <Spinner color="$accentColor" size="large" />
    </View>
  );
}
