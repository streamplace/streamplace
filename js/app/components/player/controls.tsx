import {
  Antenna,
  CheckCircle,
  ChevronLeft,
  ChevronRight,
  Circle,
  Maximize,
  Minimize,
  Settings,
  Shell,
  Sparkle,
  Squirrel,
  Star,
  Volume2,
  VolumeX,
} from "@tamagui/lucide-icons";
import { useEffect, useRef, useState } from "react";
import { Animated, Pressable } from "react-native";
import {
  Button,
  Adapt,
  H3,
  ListItem,
  Popover,
  Separator,
  Text,
  useMedia,
  View,
  XStack,
  YGroup,
} from "tamagui";
import {
  PlayerProps,
  PROTOCOL_HLS,
  PROTOCOL_PROGRESSIVE_MP4,
  PROTOCOL_PROGRESSIVE_WEBM,
  PROTOCOL_WEBRTC,
} from "./props";
import { usePlayer, usePlayerActions } from "features/player/playerSlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import Loading from "components/loading/loading";

const Bar = (props) => (
  <XStack
    height={50}
    backgroundColor="rgba(0,0,0,0.8)"
    justifyContent="space-between"
    flex-direction="row"
    opacity={props.opacity}
    animation="quick"
    animateOnly={["opacity"]}
  >
    {props.children}
  </XStack>
);

const Part = (props) => (
  <View alignItems="stretch" justifyContent="center" flexDirection="row">
    {props.children}
  </View>
);

export default function Controls(props: PlayerProps) {
  const fadeAnim = useRef(new Animated.Value(1)).current;

  // useEffect(() => {
  //   Animated.timing(fadeAnim, {
  //     toValue: props.showControls ? 1 : 1,
  //     duration: 175,
  //     useNativeDriver: false,
  //   }).start();
  // }, [fadeAnim, props.showControls]);

  let cursor = {};
  if (props.fullscreen && !props.showControls) {
    cursor = { cursor: "none" };
  }

  const onPress = () => {
    props.userInteraction();
    props.setPlayTime(Date.now());
  };

  return (
    <View
      position="absolute"
      width="100%"
      height="100%"
      zIndex={999}
      flexDirection="column"
      justifyContent="space-between"
      onPointerMove={props.userInteraction}
      onTouchStart={props.userInteraction}
      onPress={onPress}
      {...cursor}
    >
      {/* <Animated.View
        // onPointerMove={props.userInteraction}
        // onTouchStart={props.userInteraction}
        style={{
          flex: 1,
          opacity: fadeAnim,
          width: "100%",
          height: "100%",
          flexDirection: "column",
          justifyContent: "space-between",
        }}
      > */}
      <Bar opacity={props.showControls ? 1 : 0}>
        <Part>
          <View justifyContent="center" paddingLeft="$5">
            <Text>{props.name}</Text>
          </View>
        </Part>
        <Part>{/* <Text>Top Right</Text> */}</Part>
      </Bar>
      {props.ingest && <LiveBubble />}
      <Bar opacity={props.showControls ? 1 : 0}>
        <Part>
          <Pressable
            style={{
              justifyContent: "center",
            }}
            onPress={() => props.setMuted(!props.muted)}
          >
            <View paddingLeft="$5" paddingRight="$3" justifyContent="center">
              {props.muted ? <VolumeX></VolumeX> : <Volume2></Volume2>}
            </View>
          </Pressable>
        </Part>
        <Part>
          <PopoverMenu {...props} />
          <Pressable
            style={{
              justifyContent: "center",
            }}
            onPress={() => props.setFullscreen(!props.fullscreen)}
          >
            <View paddingLeft="$3" paddingRight="$5" justifyContent="center">
              {props.fullscreen ? <Minimize /> : <Maximize />}
            </View>
          </Pressable>
        </Part>
      </Bar>
      {/* </Animated.View> */}
    </View>
  );
}

export function PopoverMenu(props: PlayerProps) {
  const [open, setOpen] = useState(false);
  const media = useMedia();
  useEffect(() => {
    if (!media.sm && props.showControls === false) {
      setOpen(false);
    }
  }, [props.showControls, media.sm]);
  return (
    <Popover
      size="$5"
      allowFlip
      placement="top"
      keepChildrenMounted
      stayInFrame
      open={open}
    >
      <Popover.Trigger asChild cursor="pointer">
        <Pressable
          style={{
            justifyContent: "center",
          }}
          onPress={() => setOpen(!open)}
        >
          <View paddingLeft="$3" paddingRight="$5" justifyContent="center">
            <Settings />
          </View>
        </Pressable>
      </Popover.Trigger>

      <Adapt when="sm" platform="touch">
        <Popover.Sheet modal dismissOnSnapToBottom snapPoints={[50]}>
          <Popover.Sheet.Frame padding="$2">
            <GearMenu {...props} />
          </Popover.Sheet.Frame>
          <Popover.Sheet.Overlay
            animation="lazy"
            enterStyle={{ opacity: 0 }}
            exitStyle={{ opacity: 0 }}
          />
        </Popover.Sheet>
      </Adapt>

      <Popover.Content
        borderWidth={0}
        padding="$0"
        enterStyle={{ y: -10, opacity: 0 }}
        exitStyle={{ y: -10, opacity: 0 }}
        elevate
        userSelect="none"
        animation={[
          "quick",
          {
            opacity: {
              overshootClamping: true,
            },
          },
        ]}
      >
        <GearMenu {...props} />
      </Popover.Content>
    </Popover>
  );
}

function LiveBubble() {
  const player = useAppSelector(usePlayer());
  const dispatch = useAppDispatch();
  const { startIngest } = usePlayerActions();
  return (
    <View
      position="absolute"
      bottom={100}
      alignItems="center"
      justifyContent="center"
      width="100%"
    >
      <Button
        backgroundColor="rgba(0,0,0,0.9)"
        borderWidth={1}
        borderColor="white"
        borderRadius={9999999999}
        padding="$2"
        paddingLeft="$3"
        paddingRight="$3"
        onPress={() => {
          dispatch(startIngest(!player.ingestStarting));
        }}
      >
        <LiveBubbleText />
      </Button>
    </View>
  );
}

function LiveBubbleText() {
  const player = useAppSelector(usePlayer());
  if (!player.ingestStarting) {
    return <H3>START STREAMING</H3>;
  }
  if (player.ingestConnectionState === "connected") {
    return (
      <>
        <H3>LIVE</H3>
        <View
          backgroundColor="red"
          width={15}
          height={15}
          borderRadius={9999999999}
          marginLeft="$2"
        ></View>
      </>
    );
  }
  return <Loading />;
}

function GearMenu(props: PlayerProps) {
  const [menu, setMenu] = useState("root");
  return (
    <YGroup alignSelf="center" bordered width={240} size="$5" borderRadius="$0">
      {menu == "root" && (
        <>
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="Playback Protocol"
              subTitle="How play?"
              icon={Star}
              iconAfter={ChevronRight}
              onPress={() => setMenu("protocol")}
            />
          </YGroup.Item>
          <Separator />
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="Quality"
              subTitle="WIP"
              icon={Sparkle}
              iconAfter={ChevronRight}
            />
          </YGroup.Item>
        </>
      )}
      {menu == "protocol" && (
        <>
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="Back"
              icon={ChevronLeft}
              onPress={() => setMenu("root")}
            />
          </YGroup.Item>
          <Separator />
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="HLS"
              subTitle="HTTP Live Streaming"
              icon={Star}
              iconAfter={props.protocol === PROTOCOL_HLS ? CheckCircle : Circle}
              onPress={() => props.setProtocol(PROTOCOL_HLS)}
            />
          </YGroup.Item>
          <Separator />
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="Progressive MP4"
              subTitle="MP4 but loooong"
              icon={Shell}
              iconAfter={
                props.protocol === PROTOCOL_PROGRESSIVE_MP4
                  ? CheckCircle
                  : Circle
              }
              onPress={() => props.setProtocol(PROTOCOL_PROGRESSIVE_MP4)}
            />
          </YGroup.Item>
          <Separator />
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="Progressive WebM"
              subTitle="WebM but loooong"
              icon={Squirrel}
              iconAfter={
                props.protocol === PROTOCOL_PROGRESSIVE_WEBM
                  ? CheckCircle
                  : Circle
              }
              onPress={() => props.setProtocol(PROTOCOL_PROGRESSIVE_WEBM)}
            />
          </YGroup.Item>
          <Separator />
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="WebRTC"
              subTitle="Lowest latency, probably"
              icon={Antenna}
              iconAfter={
                props.protocol === PROTOCOL_WEBRTC ? CheckCircle : Circle
              }
              onPress={() => props.setProtocol(PROTOCOL_WEBRTC)}
            />
          </YGroup.Item>
        </>
      )}
    </YGroup>
  );
}
