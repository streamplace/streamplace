import { View } from "react-native";
import { flex, gap, layout, ml, pb, pl, px, w } from "../../ui";
import { atoms } from "../ui";
import { Code, Text } from "../ui/text";

interface SystemMessageProps {
  title: string;
  timestamp: Date;
}

export function SystemMessage({ title, timestamp }: SystemMessageProps) {
  return (
    <View style={[w.percent[100], px[2], pb[2]]}>
      <Code color="muted" tracking="widest" style={[pl[12], ml[1]]}>
        SYSTEM MESSAGE
      </Code>
      <View style={[gap.all[2], layout.flex.row]}>
        <Text
          style={{
            fontVariant: ["tabular-nums"],
            color: atoms.colors.gray[300],
          }}
        >
          {timestamp.toLocaleTimeString([], {
            hour: "2-digit",
            minute: "2-digit",
            hour12: false,
          })}
        </Text>
        <Text weight="bold" color="default" style={[flex.shrink[1]]}>
          {title}
        </Text>
      </View>
    </View>
  );
}

export default SystemMessage;
