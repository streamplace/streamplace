import { Platform, View } from "react-native";
import { place } from "streamplace";
import { SystemMessageType } from "../../lib/system-messages";
import { useTheme } from "../../lib/theme/theme";
import { bg, flex, gap, layout, ml, pb, pl, px, r, w } from "../../ui";
import { Code, Text } from "../ui/text";
import { RichTextMessage } from "./chat-message";

interface SystemMessageProps {
  variant: SystemMessageType;
  title: string;
  timestamp: Date;
  facets?: place.stream.richtext.facet.Main[];
}

export function SystemMessage({
  variant,
  title,
  timestamp,
  facets,
}: SystemMessageProps) {
  const isError = variant === SystemMessageType.command_error;
  const { theme } = useTheme();

  return (
    <View
      style={[
        w.percent[100],
        Platform.OS === "web" && px[2],
        pb[2],
        isError && bg.red[950],
        isError && r.md,
      ]}
    >
      <Code
        color="muted"
        tracking="widest"
        style={[Platform.OS === "web" ? pl[12] : pl[11], ml[1]]}
      >
        SYSTEM MESSAGE
      </Code>
      <View style={[gap.all[2], layout.flex.row]}>
        <Text
          style={{
            fontVariant: ["tabular-nums"],
            color: theme.colors.text2,
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
