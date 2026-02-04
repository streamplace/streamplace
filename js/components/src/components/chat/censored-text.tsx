import { useState } from "react";
import { Text } from "../ui/text";

export function CensoredText({ text }: { text: string }) {
  const [revealed, setRevealed] = useState(false);
  return (
    <Text
      color={revealed ? "default" : "primary"}
      onPress={() => setRevealed(!revealed)}
    >
      {revealed ? text : text.replace(/./g, "*")}
    </Text>
  );
}
