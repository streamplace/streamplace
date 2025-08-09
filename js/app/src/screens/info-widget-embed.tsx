import { ThemeProvider, zero } from "@streamplace/components";
import InformationWidget from "components/live-dashboard/information-widget";
import { View } from "react-native";

const { layout, p, flex } = zero;

export default function InfoWidgetEmbed() {
  return (
    <ThemeProvider>
      <View
        style={[
          flex.values[1],
          layout.flex.alignCenter,
          layout.flex.justifyCenter,
          p[4],
          {
            backgroundColor: "transparent",
            minHeight: "100vh",
            width: "100vw",
          },
        ]}
      >
        <InformationWidget embedMode={true} />
      </View>
    </ThemeProvider>
  );
}
