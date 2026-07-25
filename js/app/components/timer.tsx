import { Text, zero } from "@streamplace/components";
import { fontFamilies } from "@streamplace/components/src/lib/theme/tokens";
import { useEffect, useState } from "react";
import { View } from "react-native";

export default function Timer({ start }: { start: string | Date }) {
  const [elapsedTime, setElapsedTime] = useState(0);

  useEffect(() => {
    const startDate = typeof start === "string" ? new Date(start) : start;

    const interval = setInterval(() => {
      const now = new Date();
      const elapsed = Math.floor((now.getTime() - startDate.getTime()) / 1000);
      setElapsedTime(elapsed);
    }, 250);

    return () => clearInterval(interval);
  }, [start]);

  const formatTime = (seconds: number) => {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;

    return `${hours.toString().padStart(2, "0")}:${minutes.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  return (
    <View
      style={[
        { justifyContent: "center", alignItems: "center" },
        { flexDirection: "row" },
        zero.px[2],
        zero.py[1],
      ]}
    >
      <Text
        style={[
          { fontFamily: fontFamilies.monoRegular },
          {
            textShadowOffset: { width: -1, height: 1 },
            textShadowRadius: 3,
          },
        ]}
      >
        {formatTime(elapsedTime)}
      </Text>
    </View>
  );
}
