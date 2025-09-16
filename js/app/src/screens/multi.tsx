import {
  KeepAwake,
  LivestreamProvider,
  Player,
  PlayerProps,
  PlayerProvider,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import { DesktopUi } from "components/mobile/desktop-ui";
import { FullscreenProvider } from "contexts/FullscreenContext";
import { useEffect, useState } from "react";
import { Text, View } from "react-native";

const { layout, flex } = zero;

function IdViewer({ reqid }) {
  const id = usePlayerStore((p) => p.id);
  return (
    <View style={[layout.flex.center, layout.flex.row]}>
      <Text>
        {reqid} {id}
      </Text>
    </View>
  );
}

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
    <>
      <KeepAwake />
      <FullscreenProvider>
        <View style={[flex.values[1]]}>
          {rows.map((players, i) => (
            <View key={i} style={[flex.values[1], layout.flex.row]}>
              {players.map((props, j) => (
                <View key={j} style={[flex.values[1]]}>
                  {props === null ? (
                    <View />
                  ) : (
                    <LivestreamProvider src={props.src || ""}>
                      <PlayerProvider defaultId={props.playerId}>
                        <Player {...props}>
                          <DesktopUi />
                          <IdViewer reqid={props.playerId} />
                        </Player>
                      </PlayerProvider>
                    </LivestreamProvider>
                  )}
                </View>
              ))}
            </View>
          ))}
        </View>
      </FullscreenProvider>
    </>
  );
}
