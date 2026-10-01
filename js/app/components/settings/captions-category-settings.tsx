import { CaptionSettings, View, zero } from "@streamplace/components";
import { ScrollView } from "react-native";

export function CaptionsCategorySettings() {
  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[4], zero.py[4]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <CaptionSettings />
        </View>
      </View>
    </ScrollView>
  );
}
