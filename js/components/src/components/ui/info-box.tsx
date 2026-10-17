import { Text, View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import {
  fontFamilies,
  fontWeights,
  tabularNums,
  typeScale,
} from "../../lib/theme/tokens";
import * as zero from "../../ui";

const { r, layout, gap, flex } = zero;

interface InfoBoxProps {
  icon: any;
  label: string;
  value: string;
  status?: "good" | "warning" | "error" | "neutral";
}

/** Dashboard metric tile: label + icon on top, big tabular value below. */
export function InfoBox({
  icon: Icon,
  label,
  value,
  status = "neutral",
}: InfoBoxProps) {
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
        flex.values[1],
        layout.flex.column,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        r.md,
        {
          backgroundColor: c.surface2,
          borderWidth: 1,
          borderColor: c.borderSubtle,
          paddingHorizontal: 12,
          paddingVertical: 12,
          gap: 10,
          minHeight: 80,
        },
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          gap.all[3],
          zero.w.percent[100],
        ]}
      >
        <Text
          style={{
            color: c.text3,
            fontSize: typeScale.xs.fontSize,
            lineHeight: typeScale.xs.lineHeight,
            fontWeight: fontWeights.medium,
            fontFamily: fontFamilies.medium,
            letterSpacing: 0.5,
            textTransform: "uppercase",
          }}
        >
          {label}
        </Text>
        <Icon size={16} color={c.text3} />
      </View>
      <View style={[layout.flex.align.end, zero.w.percent[100]]}>
        <Text
          style={{
            color: statusColor,
            fontSize: typeScale.lg.fontSize,
            lineHeight: typeScale.lg.fontSize * 1.15,
            fontWeight: fontWeights.semibold,
            fontFamily: fontFamilies.monoSemiBold,
            ...tabularNums,
          }}
        >
          {value}
        </Text>
      </View>
    </View>
  );
}
