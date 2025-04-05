import Chat from "components/chat/chat";
import ChatBox from "components/chat/chat-box";
import Loading from "components/loading/loading";
import { Player } from "components/player/player";
import { PlayerProps } from "components/player/props";
import PlayerProvider from "components/player/provider";
import Popup from "components/popup";
import Viewers from "components/viewers";
import { usePlayer } from "features/player/playerSlice";
import {
  selectTelemetry,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import { useKeyboard } from "hooks/useKeyboard";
import usePlatform from "hooks/usePlatform";
import { useCallback, useEffect, useState } from "react";
import { LayoutChangeEvent, View as RNView, SafeAreaView } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, H2, H3, Text, useWindowDimensions, View } from "tamagui";

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

  const { src, ...extraProps } = props;
  const dispatch = useAppDispatch();
  const { width, height } = useWindowDimensions();
  const video = player.segment?.video?.[0];
  const [videoWidth, setVideoWidth] = useState(0);
  const [videoHeight, setVideoHeight] = useState(0);
  const { isKeyboardVisible, keyboardHeight } = useKeyboard();
  const { isIOS } = usePlatform();

  const [outerHeight, setOuterHeight] = useState(0);
  const [innerHeight, setInnerHeight] = useState(0);

  // this would all be really easy if i had library that would give me the
  // safe area view height and width but i don't. so let's measure
  const onInnerLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setInnerHeight(height);
  }, []);

  const onOuterLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setOuterHeight(height);
  }, []);

  useEffect(() => {
    if (video) {
      const ratio = video.width / width;
      setVideoWidth(video.width / ratio);
      setVideoHeight(video.height / ratio);
    }
  }, [video, width, height]);

  let slideKeyboard = 0;
  if (isIOS && keyboardHeight > 0) {
    slideKeyboard = -keyboardHeight + (outerHeight - innerHeight);
  }

  return (
    <RNView style={{ flex: 1 }}>
      <SafeAreaView style={{ flex: 1 }} onLayout={onOuterLayout}>
        <RNView
          style={{ flex: 1, position: "relative" }}
          onLayout={onInnerLayout}
        >
          {videoWidth === 0 && (
            <View
              f={1}
              position="absolute"
              top={0}
              left={0}
              right={0}
              bottom={0}
            >
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
                Streamplace is beta software and it helps us out to have the
                player report back on how playback is working. Would you like to
                opt in to optional player telemetry?
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
            zIndex={2}
          >
            <View
              width={videoWidth}
              height={videoHeight}
              maxHeight="100%"
              fs={0}
              $gtXs={{ fs: 1 }}
              zIndex={2}
            >
              <Player
                telemetry={telemetry === true}
                src={src}
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
              f={1}
              fg={1}
              zIndex={1}
              $gtXs={{
                width: 300,
                fb: 300,
                fs: 0,
                borderLeftColor: "#666",
                borderLeftWidth: 1,
              }}
              backgroundColor="$background2"
              animation={"quick"}
              transform={
                isIOS
                  ? [
                      {
                        translateY: slideKeyboard,
                      },
                    ]
                  : undefined
              }
            >
              <View
                $gtXs={{ display: "none" }}
                flexDirection="row"
                gap="$2"
                borderBottomColor="#666"
                borderBottomWidth={1}
                borderTopColor="#666"
                borderTopWidth={1}
                zIndex={1}
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
              <Chat />
              <View>
                <ChatBox />
              </View>
            </View>
          </View>
        </RNView>
      </SafeAreaView>
    </RNView>
  );
}
