import { Dashboard, zero } from "@streamplace/components";
import { useLiveUser } from "hooks/useLiveUser";
import { View } from "react-native";

const { layout, p, flex } = zero;

export default function InfoWidgetEmbed() {
  const isLive = useLiveUser();
  return (
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
      <Dashboard.InformationWidget embedMode={true} />
    </View>
  );
}
