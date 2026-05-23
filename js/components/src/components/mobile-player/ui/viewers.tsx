import { Eye } from "lucide-react-native";
import * as atoms from "../../../lib/theme/atoms";
import { useViewers } from "../../../livestream-store";
import { View } from "../../ui";
import ViewerCount from "./viewer-count";

export function Viewers() {
  const viewers = useViewers();
  return <DehydratedViewers viewers={viewers || 0} />;
}

export function DehydratedViewers({
  viewers,
  size = "md",
}: {
  viewers: number;
  size?: "sm" | "md";
}) {
  const iconSize = size === "sm" ? 12 : 24;
  const fontSize = size === "sm" ? 11 : 16;
  return (
    <View
      style={[
        atoms.layout.flex.center,
        atoms.layout.flex.row,
        atoms.gap.all[size === "sm" ? 1 : 2],
        atoms.px[1],
      ]}
    >
      <Eye color="#fd5050" size={iconSize} />
      <ViewerCount
        count={viewers}
        style={{ fontSize, lineHeight: fontSize * 1.5 }}
      />
    </View>
  );
}

export default Viewers;
