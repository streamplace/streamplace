import { Eye } from "lucide-react-native";
import * as atoms from "../../../lib/theme/atoms";
import { useViewers } from "../../../livestream-store";
import { View } from "../../ui";
import ViewerCount from "./viewer-count";

export function Viewers() {
  const viewers = useViewers();
  return <DehydratedViewers viewers={viewers || 0} />;
}

export function DehydratedViewers({ viewers }: { viewers: number }) {
  return (
    <View
      style={[
        atoms.layout.flex.center,
        atoms.layout.flex.row,
        atoms.gap.all[2],
        atoms.px[1],
      ]}
    >
      <Eye color="#fd5050" />
      <ViewerCount count={viewers} />
    </View>
  );
}

export default Viewers;
