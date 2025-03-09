import { Player } from "components/player/player";
import Popup from "components/popup";
import {
  selectTelemetry,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import { Button, View, Text } from "tamagui";
import { useAppSelector, useAppDispatch } from "store/hooks";
import { H3, H1 } from "tamagui";
import { PlayerProps } from "components/player/props";
import PlayerProvider from "components/player/provider";

export default function Livestream(props: Partial<PlayerProps>) {
  return (
    <PlayerProvider {...props}>
      <LivestreamInner {...props} />
    </PlayerProvider>
  );
}

export function LivestreamInner(props: Partial<PlayerProps>) {
  const telemetry = useAppSelector(selectTelemetry);
  const { src, protocol, ...extraProps } = props;
  const dispatch = useAppDispatch();
  return (
    <View f={1} position="relative">
      {telemetry === null && (
        <Popup
          onClose={() => {
            dispatch(telemetryOpt(false));
          }}
          containerProps={{
            bottom: "$8",
            zIndex: 1000,
          }}
          bubbleProps={{
            cursor: "pointer",
            backgroundColor: "$accentBackground",
            gap: "$3",
            maxWidth: 400,
          }}
        >
          <H3 textAlign="center">Player Telemetry</H3>
          <Text>
            Streamplace is beta software and it helps us out to have the player
            report back on how playback is working. Would you like to opt in to
            optional player telemetry?
          </Text>
          <View flexDirection="row" gap="$2" f={1}>
            <Button
              f={3}
              backgroundColor="$accentColor"
              onPress={() => {
                dispatch(telemetryOpt(true));
              }}
            >
              Opt in
            </Button>
            <Button
              f={3}
              onPress={() => {
                dispatch(telemetryOpt(false));
              }}
            >
              Opt out
            </Button>
          </View>
        </Popup>
      )}
      <View f={1} flexDirection="row">
        <View f={1}>
          <Player
            telemetry={telemetry === true}
            src={src}
            forceProtocol={protocol}
            {...extraProps}
          />
          <View height={100} fg={0} p="$4">
            <H1>Stream Title Goes Here</H1>
          </View>
        </View>
        <View width={300} backgroundColor="$background2" p="$4">
          <Text>this chat</Text>
        </View>
      </View>
    </View>
  );
}
