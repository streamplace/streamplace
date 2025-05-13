import { View, Text } from "tamagui";
import { useState, useEffect } from "react";

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
      justifyContent="center"
      flexDirection="row"
      alignItems="center"
      paddingHorizontal="$2"
      paddingVertical="$1"
    >
      <Text
        fontFamily="$mono"
        textShadowOffset={{ width: -1, height: 1 }}
        textShadowRadius={3}
      >
        {formatTime(elapsedTime)}
      </Text>
    </View>
  );
}
