import { Text, View } from "react-native";
import * as zero from "../../ui";

const { text, layout, py, gap } = zero;

interface InfoRowProps {
  icon: any;
  label: string;
  value: string;
  status?: "good" | "warning" | "error" | "neutral";
}

export function InfoRow({
  icon: Icon,
  label,
  value,
  status = "neutral",
}: InfoRowProps) {
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
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        py[2],
      ]}
    >
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        <Icon size={16} color="#9ca3af" />
        <Text style={[text.gray[300], { fontSize: 13, fontWeight: "500" }]}>
          {label}
        </Text>
      </View>
      <Text style={[statusColor, { fontSize: 13, fontWeight: "600" }]}>
        {value}
      </Text>
    </View>
  );
}
