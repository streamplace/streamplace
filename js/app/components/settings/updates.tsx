import pkg from "../../package.json";
import { View, H2 } from "tamagui";

// maybe someday some PWA update stuff will live here
export function Updates() {
  return (
    <View alignItems="center" justifyContent="center" py="$6">
      <View>
        <H2 textAlign="center">Streamplace v{pkg.version}</H2>
      </View>
    </View>
  );
}
