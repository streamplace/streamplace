import { Player } from "components";
import { PlayerProps } from "components/player/props";
import { useEffect, useState } from "react";
import { Text, View, XStack, YStack } from "tamagui";

export default function MultiScreen({ route }) {
  const config = route.params?.config;
  if (typeof config !== "string") {
    return <View />;
  }

  const [rows, setRows] = useState<Partial<PlayerProps | null>[][]>([]);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    try {
      let nearestSquareExpo = 1;
      const playerProps = JSON.parse(
        config as string,
      ) as Partial<PlayerProps>[];
      while (Math.pow(nearestSquareExpo, 2) < playerProps.length) {
        nearestSquareExpo += 1;
      }
      const rows: Partial<PlayerProps | null>[][] = [];
      let idx = 0;
      for (let i = 0; i < nearestSquareExpo; i += 1) {
        const row: Partial<PlayerProps | null>[] = [];
        for (let j = 0; j < nearestSquareExpo; j += 1) {
          if (playerProps[idx]) {
            row.push(playerProps[idx]);
          } else {
            row.push(null);
          }
          idx += 1;
        }
        rows.push(row);
      }
      setRows(rows);
    } catch (e) {
      setError(e.message);
    }
  }, [config]);
  if (error) {
    return <Text>{error}</Text>;
  }
  return (
    <YStack f={1} fb={0}>
      {rows.map((players, i) => (
        <XStack key={i} f={1} fb={0}>
          {players.map((props, j) => (
            <View key={j} f={1} fb={0}>
              {props === null ? <View /> : <Player {...props}></Player>}
            </View>
          ))}
        </XStack>
      ))}
    </YStack>
  );
}
