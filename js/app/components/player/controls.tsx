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
import { Animated, ImageBackground, Pressable } from "react-native";
import {
  Button,
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
  Slider,
  Adapt,
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
import usePlatform from "hooks/usePlatform";
import { Image } from "tamagui";

const Bar = (props) => (
  <XStack
    height={50}
    backgroundColor="rgba(0,0,0,0.8)"
    justifyContent="space-between"
    alignItems="stretch"
    flex-direction="row"
    animation="quick"
    animateOnly={["opacity"]}
    {...props}
  />
);

const Part = (props) => (
  <View
    alignItems="stretch"
    justifyContent="center"
    flexDirection="row"
    flexBasis={0}
    flexGrow={1}
    {...props}
  />
);

const VolumeSlider = ({
  volume,
  setVolume,
  muted,
  setMuted,
  showControls,
}: {
  volume: number;
  setVolume: (volume: number) => void;
  muted: boolean;
  setMuted: (muted: boolean) => void;
  showControls: boolean;
}) => {
  const [open, setOpen] = useState(false);
  const [sliderValue, setSliderValue] = useState(volume);
  const media = useMedia();
  const { isWeb } = usePlatform();

  // Update slider value when volume prop changes
  useEffect(() => {
    setSliderValue(muted ? 0 : volume);
  }, [volume, muted]);

  // Close popover when controls are hidden
  useEffect(() => {
    if (!media.sm && !showControls) {
      setOpen(false);
    }
  }, [showControls, media.sm]);

  // Reset slider value when popover opens
  useEffect(() => {
    if (open) {
      setSliderValue(muted ? 0 : volume);
    }
  }, [open, volume, muted]);

  const handleValueChange = (value) => {
    const newValue = value[0];
    setSliderValue(newValue);

    // Update volume and mute state
    if (newValue === 0) {
      setMuted(true);
    } else {
      setVolume(newValue);
      if (muted) {
        setMuted(false);
      }
    }
  };

  if (!isWeb) {
    return (
      <View paddingLeft="$5" paddingRight="$3" justifyContent="center">
        <Pressable onPress={() => setMuted(!muted)}>
          {muted ? <VolumeX size={20} /> : <Volume2 size={20} />}
        </Pressable>
      </View>
    );
  }

  return (
    <Popover
      size="$5"
      allowFlip
      placement="top"
      open={open}
      onOpenChange={setOpen}
    >
      <Popover.Trigger asChild cursor="pointer">
        <View paddingLeft="$5" paddingRight="$3" justifyContent="center">
          <Pressable onPress={() => setOpen(!open)}>
            {muted ? <VolumeX size={20} /> : <Volume2 size={20} />}
          </Pressable>
        </View>
      </Popover.Trigger>

      <Popover.Content
        borderWidth={0}
        padding="$3"
        enterStyle={{ y: 10, opacity: 0 }}
        exitStyle={{ y: 10, opacity: 0 }}
        elevate
        animation={[
          "quick",
          {
            opacity: {
              overshootClamping: true,
            },
          },
        ]}
      >
        <View width={50} height={150} alignItems="center" gap="$3">
          <XStack alignItems="center" gap="$2">
            <Pressable onPress={() => setMuted(!muted)}>
              {muted ? <VolumeX size={20} /> : <Volume2 size={20} />}
            </Pressable>
            <Text>{Math.round(sliderValue * 100)}</Text>
          </XStack>
          <View
            height={100}
            width="100%"
            alignItems="center"
            justifyContent="center"
          >
            <Slider
              size="$2"
              orientation="vertical"
              height="100%"
              value={[sliderValue]}
              onValueChange={handleValueChange}
              min={0}
              max={1}
              step={0.01}
            >
              <Slider.Track backgroundColor="$gray8" width={4}>
                <Slider.TrackActive backgroundColor="$gray5" />
              </Slider.Track>
              <Slider.Thumb
                circular
                index={0}
                size="$1"
                backgroundColor="white"
              />
            </Slider>
          </View>
        </View>
      </Popover.Content>
    </Popover>
  );
};

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
  const m = useMedia();

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
      {props.muteWasForced && (
        <View
          position="absolute"
          left={0}
          bottom={0}
          padding={20}
          opacity={props.showControls ? 0 : 1}
        >
          <VolumeX size={60} color="red" />
        </View>
      )}
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
      <Bar
        opacity={props.showControls ? (props.fullscreen ? 0 : 1) : 0}
        cursor={props.embedded ? "pointer" : undefined}
        onPress={() => {
          if (props.embedded) {
            // Open the current URL in a new window
            const u = new URL(window.location.href);
            u.pathname = u.pathname.replace("/embed", "");
            window.open(u.toString(), "_blank");
            props.setMuted(true);
          }
        }}
      >
        <Part justifyContent="flex-start" overflow="hidden">
          <View justifyContent="center" paddingLeft="$5" maxWidth="100%">
            <Text wordWrap="break-word" numberOfLines={1} ellipsizeMode="tail">
              {props.name}
            </Text>
          </View>
        </Part>
        <Part>
          {props.embedded && m.gtXs ? (
            <>
              <Image
                src={require("../../assets/images/cube_small.png")}
                height={50}
                width={50}
              />
            </>
          ) : null}
        </Part>
        <Part justifyContent="flex-end">
          <Viewers viewers={player.viewers ?? 0} />
        </Part>
      </Bar>
      {props.ingest && <LiveBubble />}
      <Bar opacity={props.showControls ? 1 : 0}>
        <Part justifyContent="flex-start">
          <VolumeSlider
            volume={props.volume}
            setVolume={(vol) => {
              props.setVolume(vol);
              props.setMuteWasForced(false);
            }}
            muted={props.muted}
            showControls={props.showControls}
            setMuted={(muted) => {
              dispatch(userMute(muted));
              props.setMuteWasForced(false);
              props.setMuted(muted);
            }}
          />
        </Part>
        <Part justifyContent="flex-end">
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
      setSelectedRendition={(rendition) => {
        dispatch(setSelectedRendition(rendition));
        setOpen(false);
      }}
      setProtocol={(protocol) => {
        dispatch(setProtocol(protocol));
        setOpen(false);
      }}
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
      onOpenChange={setOpen}
    >
      <Popover.Trigger asChild cursor="pointer">
        <View position="relative" justifyContent="center" height={50}>
          <Pressable
            style={{
              justifyContent: "center",
              height: "100%",
            }}
            onPress={() => setOpen(!open)}
          >
            <View paddingLeft="$3" paddingRight="$5" justifyContent="center">
              <Settings />
            </View>
          </Pressable>
        </View>
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
          <Separator />
          {/* <YGroup.Item>
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
          </YGroup.Item>
          <Separator /> */}
          <YGroup.Item>
            <ListItem
              hoverTheme
              pressTheme
              title="WebRTC"
              subTitle="Lowest latency, probably"
              icon={Antenna}
              iconAfter={protocol === PROTOCOL_WEBRTC ? CheckCircle : Circle}
              onPress={() => setProtocol(PROTOCOL_WEBRTC)}
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
                  onPress={() => setSelectedRendition("auto")}
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
              onPress={() => setSelectedRendition("source")}
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
                  onPress={() => setSelectedRendition(rendition.name)}
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
