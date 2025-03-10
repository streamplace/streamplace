import { Player } from "components/player/player";
import Popup from "components/popup";
import {
  selectTelemetry,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import { Button, View, Text, H2 } from "tamagui";
import { useAppSelector, useAppDispatch } from "store/hooks";
import { H3 } from "tamagui";
import { PlayerProps } from "components/player/props";
import PlayerProvider from "components/player/provider";
import Chat from "components/chat/chat";
import { usePlayer } from "features/player/playerSlice";

export default function Livestream(props: Partial<PlayerProps>) {
  return (
    <PlayerProvider {...props}>
      <LivestreamInner {...props} />
    </PlayerProvider>
  );
}

export function LivestreamInner(props: Partial<PlayerProps>) {
  const telemetry = useAppSelector(selectTelemetry);
  const player = useAppSelector(usePlayer());
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
      <View f={1} flexDirection="column" $gtXs={{ flexDirection: "row" }}>
        <View f={1} fs={0} $gtXs={{ fs: 1 }}>
          <Player
            telemetry={telemetry === true}
            src={src}
            forceProtocol={protocol}
            {...extraProps}
          />
          <View
            height={100}
            fg={0}
            p="$4"
            display="none"
            $gtXs={{ display: "flex" }}
          >
            <H2>{player.livestream?.["place.stream.livestream"]?.title}</H2>
          </View>
        </View>
        <View
          f={1}
          fg={1}
          $gtXs={{ width: 300, fb: 300, fs: 0 }}
          backgroundColor="$background2"
        >
          <Chat />
        </View>
      </View>
    </View>
  );
}
