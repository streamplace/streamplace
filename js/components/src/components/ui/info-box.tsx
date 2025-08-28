import { Text, View } from "react-native";
import * as zero from "../../ui";

const { bg, r, p, text, layout, gap, flex } = zero;

interface InfoBoxProps {
  icon: any;
  label: string;
  value: string;
  status?: "good" | "warning" | "error" | "neutral";
}

export function InfoBox({
  icon: Icon,
  label,
  value,
  status = "neutral",
}: InfoBoxProps) {
  const statusColors = {
    good: text.green[400],
    warning: text.yellow[400],
    error: text.red[400],
    neutral: text.white,
  };

  const statusColor = statusColors[status];

  return (
    <View
      style={[
        flex.values[1],
        layout.flex.column,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        bg.neutral[700],
        r.sm,
        p[2],
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
        <Text style={[text.gray[100], { fontSize: 13, fontWeight: "500" }]}>
          {label}
        </Text>
        <Icon size={16} color="#9ca3af" />
      </View>
      <View style={[layout.flex.align.end, zero.w.percent[100]]}>
        <Text style={[statusColor, { fontSize: 26, fontWeight: "600" }]}>
          {value}
        </Text>
      </View>
    </View>
  );
}
