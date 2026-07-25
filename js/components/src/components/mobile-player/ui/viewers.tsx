import { Eye } from "lucide-react-native";
import { useViews } from "../../..";
import * as atoms from "../../../lib/theme/atoms";
import { usePlayerStore } from "../../../player-store";
import { View } from "../../ui";
import ViewerCount from "./viewer-count";

// red reads as "live"; anything else (VOD total views) shouldn't
const LIVE_COLOR = "#fd5050";
const VIEWS_COLOR = "#fff";

export function Viewers() {
  const views = useViews();
  const mode = usePlayerStore((x) => x.mode);
  const color = mode === "live" ? LIVE_COLOR : VIEWS_COLOR;
  return <DehydratedViewers viewers={views || 0} color={color} />;
}

export function DehydratedViewers({
  viewers,
  color = LIVE_COLOR,
  size = "md",
}: {
  viewers: number;
  color?: string;
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
      <Eye color={color} size={iconSize} />
      <ViewerCount
        count={viewers}
        style={{ fontSize, lineHeight: fontSize * 1.5 }}
      />
    </View>
  );
}

export default Viewers;
