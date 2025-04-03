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
  Star,
  Volume2,
  VolumeX,
} from "@tamagui/lucide-icons";
import { Dispatch, Fragment, useEffect, useRef, useState } from "react";
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
  H1,
  H5,
  Paragraph,
} from "tamagui";
import { PlayerProps, PROTOCOL_HLS, PROTOCOL_WEBRTC } from "./props";
import {
  usePlayer,
  usePlayerActions,
  usePlayerProtocol,
  usePlayerRenditions,
  usePlayerSegment,
  usePlayerSelectedRendition,
} from "features/player/playerSlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import Loading from "components/loading/loading";
import Viewers from "components/viewers";
import { userMute } from "features/streamplace/streamplaceSlice";
import { Countdown } from "components/countdown";
import { Rendition } from "lexicons/types/place/stream/defs";

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

  const player = useAppSelector(usePlayer());
  const dispatch = useAppDispatch();

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
      {!props.offline ? null : (
        <View
          position="absolute"
          width="100%"
          backgroundColor="black"
          height="100%"
          flex={1}
          justifyContent="center"
          alignItems="center"
          zIndex={1000}
        >
          <Offline />
        </View>
      )}
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
        <Part>
          <Viewers viewers={player.viewers ?? 0} />
        </Part>
      </Bar>
      {props.ingest && <LiveBubble />}
      <Bar opacity={props.showControls ? 1 : 0}>
        <Part>
          <Pressable
            style={{
              justifyContent: "center",
            }}
            onPress={() => {
              dispatch(userMute(!props.muted));
              props.setMuted(!props.muted);
            }}
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
  const renditions = useAppSelector(usePlayerRenditions());
  const selectedRendition = useAppSelector(usePlayerSelectedRendition());
  const protocol = useAppSelector(usePlayerProtocol());
  const { setSelectedRendition, setProtocol } = usePlayerActions();
  const dispatch = useAppDispatch();
  // on android, this appears to lose its context. idk. so we just pass everything through.
  const gearMenu = (
    <GearMenu
      {...props}
      renditions={renditions}
      selectedRendition={selectedRendition ?? "source"}
      protocol={protocol}
      setSelectedRendition={setSelectedRendition}
      setProtocol={setProtocol}
      dispatch={dispatch}
    />
  );
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
          <Popover.Sheet.Frame padding="$2">{gearMenu}</Popover.Sheet.Frame>
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
        {gearMenu}
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

function GearMenu(
  props: PlayerProps & {
    renditions: Rendition[];
    selectedRendition: string;
    protocol: string;
    setSelectedRendition: (rendition: string) => void;
    setProtocol: (protocol: string) => void;
    dispatch: Dispatch<any>;
  },
) {
  const [menu, setMenu] = useState("root");
  const {
    renditions,
    selectedRendition,
    protocol,
    setSelectedRendition,
    setProtocol,
    dispatch,
  } = props;

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
              subTitle="Adjust bandwidth usage"
              icon={Sparkle}
              iconAfter={ChevronRight}
              onPress={() => setMenu("quality")}
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
              iconAfter={protocol === PROTOCOL_HLS ? CheckCircle : Circle}
              onPress={() => dispatch(setProtocol(PROTOCOL_HLS))}
            />
          </YGroup.Item>
          {/* <Separator />
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="Progressive MP4"
              subTitle="MP4 but loooong"
              icon={Shell}
              iconAfter={
                protocol === PROTOCOL_PROGRESSIVE_MP4 ? CheckCircle : Circle
              }
              onPress={() => dispatch(setProtocol(PROTOCOL_PROGRESSIVE_MP4))}
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
                protocol === PROTOCOL_PROGRESSIVE_WEBM ? CheckCircle : Circle
              }
              onPress={() => dispatch(setProtocol(PROTOCOL_PROGRESSIVE_WEBM))}
            />
          </YGroup.Item> */}
          <Separator />
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="WebRTC"
              subTitle="Lowest latency, probably"
              icon={Antenna}
              iconAfter={protocol === PROTOCOL_WEBRTC ? CheckCircle : Circle}
              onPress={() => dispatch(setProtocol(PROTOCOL_WEBRTC))}
            />
          </YGroup.Item>
        </>
      )}
      {menu == "quality" && (
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
          {protocol === PROTOCOL_HLS && (
            <>
              <YGroup.Item>
                <ListItem
                  hoverTheme
                  pressTheme
                  title="Auto"
                  subTitle="Automatic with HLS"
                  icon={Star}
                  iconAfter={
                    props.selectedRendition === "auto" ? CheckCircle : Circle
                  }
                  onPress={() => dispatch(setSelectedRendition("auto"))}
                />
              </YGroup.Item>
              <Separator />
            </>
          )}
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="Source"
              subTitle="Original quality"
              icon={Star}
              iconAfter={
                props.selectedRendition === "source" ? CheckCircle : Circle
              }
              onPress={() => dispatch(setSelectedRendition("source"))}
            />
          </YGroup.Item>
          {renditions.map((rendition) => (
            <Fragment key={rendition.name}>
              <Separator />
              <YGroup.Item>
                <ListItem
                  hoverTheme
                  pressTheme
                  title={rendition.name}
                  subTitle={rendition.name}
                  icon={Shell}
                  iconAfter={
                    selectedRendition === rendition.name ? CheckCircle : Circle
                  }
                  onPress={() => dispatch(setSelectedRendition(rendition.name))}
                />
              </YGroup.Item>
            </Fragment>
          ))}
        </>
      )}
    </YGroup>
  );
}

export function Offline() {
  const segment = useAppSelector(usePlayerSegment());
  return (
    <View flex={1} justifyContent="center" alignItems="center">
      <View flexDirection="row">
        <H1 paddingRight="$3">Offline</H1>
      </View>
      {segment && (
        <>
          <Paragraph>Playback will start automatically</Paragraph>
          <H5>Last seen:</H5>
          <Countdown from={segment.startTime} small={true} />
        </>
      )}
    </View>
  );
}
