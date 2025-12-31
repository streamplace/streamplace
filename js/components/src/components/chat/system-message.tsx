import { View } from "react-native";
import { Main } from "streamplace/src/lexicons/types/place/stream/richtext/facet";
import { SystemMessageType } from "../../lib/system-messages";
import { colors, flex, gap, layout, ml, pb, pl, px, w } from "../../ui";
import { Code, Text } from "../ui/text";
import { RichTextMessage } from "./chat-message";

interface SystemMessageProps {
  variant: SystemMessageType;
  title: string;
  timestamp: Date;
  facets?: Main[];
}

export function SystemMessage({
  variant,
  title,
  timestamp,
  facets,
}: SystemMessageProps) {
  return (
    <View style={[w.percent[100], px[2], pb[2]]}>
      <Code color="muted" tracking="widest" style={[pl[12], ml[1]]}>
        SYSTEM MESSAGE
      </Code>
      <View style={[gap.all[2], layout.flex.row]}>
        <Text
          style={{
            fontVariant: ["tabular-nums"],
            color: colors.gray[400],
          }}
        >
          {timestamp.toLocaleTimeString([], {
            hour: "2-digit",
            minute: "2-digit",
            hour12: false,
          })}
        </Text>
        <Text weight="bold" color="default" style={[flex.shrink[1]]}>
          <RichTextMessage facets={facets} text={title} />
        </Text>
      </View>
    </View>
  );
}

export default SystemMessage;
