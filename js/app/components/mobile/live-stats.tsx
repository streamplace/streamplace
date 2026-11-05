import { useTranslation, useViews, View, zero } from "@streamplace/components";
import ViewerCount from "@streamplace/components/src/components/mobile-player/ui/viewer-count";
import {
  colors,
  spacing,
  tabularNums,
  typeScale,
} from "@streamplace/components/src/lib/theme/tokens";
import { useLiveStatus } from "hooks/useLiveStatus";
import { UsersRound } from "lucide-react-native";

export function LiveStats() {
  const { isLive } = useLiveStatus();
  const viewers = useViews();
  const { t } = useTranslation("common");
  if (!isLive || viewers === null) return null;

  return (
    <View
      accessible
      accessibilityLabel={t("watching-count", { count: viewers })}
      pointerEvents="none"
      style={[
        zero.layout.flex.row,
        zero.layout.flex.alignCenter,
        zero.gap.all[1],
        zero.pl[2],
      ]}
    >
      <UsersRound size={spacing[4]} color={colors.white} />
      <ViewerCount
        count={viewers}
        style={{ ...typeScale.sm, ...tabularNums, color: colors.white }}
      />
    </View>
  );
}
