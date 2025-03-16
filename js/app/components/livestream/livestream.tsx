import { Player } from "components/player/player";
import Popup from "components/popup";
import {
  selectTelemetry,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import { Button, View, Text, H2, useWindowDimensions } from "tamagui";
import { useAppSelector, useAppDispatch } from "store/hooks";
import { H3 } from "tamagui";
import { PlayerProps } from "components/player/props";
import PlayerProvider from "components/player/provider";
import Chat from "components/chat/chat";
import { usePlayer } from "features/player/playerSlice";
import { useState, useEffect } from "react";
import Loading from "components/loading/loading";
import Viewers from "components/viewers";
import ChatBox from "components/chat/chat-box";
import { useKeyboard } from "hooks/useKeyboard";

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
  const { width, height } = useWindowDimensions();
  const video = player.segment?.video?.[0];
  const [videoWidth, setVideoWidth] = useState(0);
  const [videoHeight, setVideoHeight] = useState(0);
  const { isKeyboardVisible } = useKeyboard();
  useEffect(() => {
    if (video) {
      const ratio = video.width / width;
      setVideoWidth(video.width / ratio);
      setVideoHeight(video.height / ratio);
    }
  }, [video, width, height]);

  return (
    <View f={1} position="relative">
      {videoWidth === 0 && (
        <View f={1} position="absolute" top={0} left={0} right={0} bottom={0}>
          <Loading />
        </View>
      )}
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
      <View
        f={1}
        opacity={videoWidth === 0 ? 0 : 1}
        flexDirection="column"
        $gtXs={{ flexDirection: "row" }}
      >
        <View width={videoWidth} height={videoHeight} fs={0} $gtXs={{ fs: 1 }}>
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
            flexDirection="row"
            alignItems="flex-start"
            justifyContent="space-between"
            $gtXs={{ display: "flex" }}
          >
            <H2>{player.livestream?.record.title}</H2>
            <View justifyContent="center" paddingRight="$3">
              <Viewers viewers={player.viewers ?? 0} />
            </View>
          </View>
        </View>
        <View
          $gtXs={{ display: "none" }}
          flexDirection="row"
          gap="$2"
          borderBottomColor="#666"
          borderBottomWidth={1}
          display={isKeyboardVisible ? "none" : "flex"}
          borderTopColor="#666"
          borderTopWidth={1}
        >
          <View f={1} fb={0} padding="$3" justifyContent="center">
            <Text fontSize={18} numberOfLines={1} ellipsizeMode="tail">
              {player.livestream?.record.title}
            </Text>
          </View>
          <View justifyContent="center" paddingRight="$3">
            <Viewers viewers={player.viewers ?? 0} />
          </View>
        </View>
        <View
          f={1}
          fg={1}
          $gtXs={{
            width: 300,
            fb: 300,
            fs: 0,
            borderLeftColor: "#666",
            borderLeftWidth: 1,
          }}
          backgroundColor="$background2"
        >
          <Chat />
          <View>
            <ChatBox />
          </View>
        </View>
      </View>
    </View>
  );
}
