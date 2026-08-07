import { Text, View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import {
  fontFamilies,
  fontWeights,
  tabularNums,
  typeScale,
} from "../../lib/theme/tokens";
import * as zero from "../../ui";

const { layout, py, gap } = zero;

interface InfoRowProps {
  icon: any;
  label: string;
  value: string;
  status?: "good" | "warning" | "error" | "neutral";
}

/** Dashboard row: icon + label left, tabular status value right. */
export function InfoRow({
  icon: Icon,
  label,
  value,
  status = "neutral",
}: InfoRowProps) {
  const { theme } = useTheme();
  const c = theme.colors;

  const statusColor = {
    good: c.success,
    warning: c.warning,
    error: c.danger,
    neutral: c.text1,
  }[status];

  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        py[2],
      ]}
    >
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        <Icon size={16} color={c.text3} />
        <Text
          style={{
            color: c.text2,
            fontSize: typeScale.sm.fontSize,
            lineHeight: typeScale.sm.lineHeight,
            fontWeight: fontWeights.medium,
            fontFamily: fontFamilies.medium,
          }}
        >
          {label}
        </Text>
      </View>
      <Text
        style={{
          color: statusColor,
          fontSize: typeScale.sm.fontSize,
          lineHeight: typeScale.sm.lineHeight,
          fontWeight: fontWeights.semibold,
          fontFamily: fontFamilies.semiBold,
          ...tabularNums,
        }}
      >
        {value}
      </Text>
    </View>
  );
}
