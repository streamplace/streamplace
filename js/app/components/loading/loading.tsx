import { Spinner as TamaguiSpinner, View } from "tamagui";

export default function () {
  return (
    <View f={1} alignItems="center" justifyContent="center">
      <Spinner />
    </View>
  );
}

export function Spinner() {
  return <TamaguiSpinner color="$accentColor" size="large" />;
}
