import pkg from "../../package.json";
import { View, H2 } from "tamagui";

// maybe someday some PWA update stuff will live here
export function Updates() {
  return (
    <View
      f={1}
      alignItems="center"
      justifyContent="center"
      fg={1}
      flexBasis={0}
    >
      <View>
        <H2 textAlign="center">Streamplace v{pkg.version}</H2>
      </View>
    </View>
  );
}
