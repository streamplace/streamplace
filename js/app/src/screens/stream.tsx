import { Player } from "components/player/player";
import { PlayerProps } from "components/player/props";
import { Button, H3, isWeb, Text, View } from "tamagui";
import { queryToProps } from "./util";
import Popup from "components/popup";
import {
  selectTelemetry,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import { useAppDispatch, useAppSelector } from "store/hooks";

export default function StreamScreen({ route }) {
  const { user, protocol, url } = route.params;
  let extraProps: Partial<PlayerProps> = {};
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  let src = user;
  if (user === "stream") {
    src = url;
  }
  const telemetry = useAppSelector(selectTelemetry);
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
      <Player
        telemetry={telemetry === true}
        src={src}
        forceProtocol={protocol}
        {...extraProps}
      />
    </View>
  );
}
