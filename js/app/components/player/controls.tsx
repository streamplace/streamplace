import {
  intoPlayerProtocol,
  usePlayerStore,
  useRenditions,
  useSegment,
  useViewers,
} from "@streamplace/components";
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
import { Countdown } from "components/countdown";
import Loading from "components/loading/loading";
import Viewers from "components/viewers";
import {
  Dispatch,
  Fragment,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { Animated, Platform, Pressable } from "react-native";
import { useAppDispatch } from "store/hooks";
import { PlaceStreamDefs } from "streamplace";
import {
  Adapt,
  Button,
  H1,
  H3,
  H5,
  Image,
  ListItem,
  Paragraph,
  Popover,
  Separator,
  Slider,
  Text,
  useMedia,
  View,
  XStack,
  YGroup,
} from "tamagui";

const PROTOCOL_HLS = "hls";
// const PROTOCOL_PROGRESSIVE_MP4 = "progressive-mp4";
// const PROTOCOL_PROGRESSIVE_WEBM = "progressive-webm";
const PROTOCOL_WEBRTC = "webrtc";

const Bar = (props) => (
  <XStack
    paddingLeft="$3"
    paddingRight="$3"
    paddingTop="$3"
    paddingBottom="$3"
    backgroundColor="rgba(0,0,0,0.5)"
    position="relative"
    minHeight={60}
    {...props}
  />
);

const Part = (props) => (
  <XStack
    flex={1}
    alignItems="center"
    justifyContent="center"
    pointerEvents="auto"
    {...props}
  />
);

const VolumeSlider = (props: {
  showControls: boolean;
  playerId: string | undefined;
}) => {
  const muted = usePlayerStore((state) => state.muted, props.playerId);
  const setMuted = usePlayerStore((state) => state.setMuted, props.playerId);
  const volume = usePlayerStore((state) => state.volume, props.playerId);
  const setVolume = usePlayerStore((state) => state.setVolume, props.playerId);

  const [volumeVisible, setVolumeVisible] = useState(false);
  const [volumeSliderWidth, setVolumeSliderWidth] = useState(0);
  const [localVolume, setLocalVolume] = useState(volume);
  const sliderWidth = volumeVisible ? volumeSliderWidth : 0;
  const sliderOpacity = volumeVisible ? 1 : 0;

  const volumeSliderRef = useRef<HTMLDivElement>(null);

  const fadeAnim = useRef(new Animated.Value(sliderOpacity)).current;
  const widthAnim = useRef(new Animated.Value(sliderWidth)).current;

  useEffect(() => {
    Animated.parallel([
      Animated.timing(fadeAnim, {
        toValue: sliderOpacity,
        duration: 175,
        useNativeDriver: false,
      }),
      Animated.timing(widthAnim, {
        toValue: sliderWidth,
        duration: 175,
        useNativeDriver: false,
      }),
    ]).start();
  }, [fadeAnim, widthAnim, sliderOpacity, sliderWidth]);

  useEffect(() => {
    if (volumeSliderRef.current) {
      const rect = volumeSliderRef.current.getBoundingClientRect();
      setVolumeSliderWidth(rect.width);
    }
  }, [volumeSliderRef]);

  useEffect(() => {
    setLocalVolume(volume);
  }, [volume]);

  const handleVolumeChange = useCallback(
    (volume: number[]) => {
      const newVolume = volume[0];
      setLocalVolume(newVolume);
      setVolume(newVolume);
    },
    [setVolume],
  );

  const handleMuteToggle = useCallback(() => {
    setMuted(!muted);
  }, [muted, setMuted]);

  return (
    <XStack
      alignItems="center"
      onPointerEnter={() => setVolumeVisible(true)}
      onPointerLeave={() => setVolumeVisible(false)}
      height={50}
      paddingRight="$3"
    >
      <Pressable
        onPress={handleMuteToggle}
        style={{
          justifyContent: "center",
          height: "100%",
        }}
      >
        <View paddingLeft="$3" paddingRight="$3" justifyContent="center">
          <Text>{muted ? <VolumeX /> : <Volume2 />}</Text>
        </View>
      </Pressable>
      {Platform.OS === "web" && (
        <Animated.View
          style={{
            opacity: fadeAnim,
            width: widthAnim,
          }}
        >
          <View
            ref={volumeSliderRef}
            width={120}
            paddingRight="$3"
            justifyContent="center"
            zi={20}
          >
            <Slider
              size="$2"
              value={[localVolume]}
              max={1}
              step={0.01}
              onValueChange={handleVolumeChange}
              zi={20}
            >
              <Slider.Track>
                <Slider.TrackActive />
              </Slider.Track>
              <Slider.Thumb circular index={0} />
            </Slider>
          </View>
        </Animated.View>
      )}
    </XStack>
  );
};

function isRefObject(
  ref: any,
): ref is
  | React.RefObject<HTMLVideoElement>
  | React.MutableRefObject<HTMLVideoElement | null> {
  return ref && typeof ref === "object" && "current" in ref;
}

export default function Controls(props: { name: string; playerId?: string }) {
  const playerId = props.playerId;

  const fullscreen = usePlayerStore((state) => state.fullscreen, playerId);
  const setFullscreen = usePlayerStore(
    (state) => state.setFullscreen,
    playerId,
  );
  const setMuted = usePlayerStore((state) => state.setMuted, props.playerId);
  const showControls = usePlayerStore((state) => state.showControls, playerId);
  const setPlayTime = usePlayerStore((state) => state.setPlayTime, playerId);
  const offline = usePlayerStore((state) => state.offline, playerId);
  const muteWasForced = usePlayerStore(
    (state) => state.muteWasForced,
    playerId,
  );
  const embedded = usePlayerStore((state) => state.embedded, playerId);
  const videoRef = usePlayerStore((state) => state.videoRef, playerId);
  const setUserInteraction = usePlayerStore(
    (state) => state.setUserInteraction,
    playerId,
  );
  const isIngesting = usePlayerStore((x) => x.ingestConnectionState !== null);
  const pipAction = usePlayerStore((x) => x.pipAction);

  const fadeAnim = useRef(new Animated.Value(1)).current;

  let cursor = {};
  if (fullscreen && !showControls) {
    cursor = { cursor: "none" };
  }

  const onPress = () => {
    setUserInteraction();
    setPlayTime(Date.now());
  };

  const viewers = useViewers();
  const m = useMedia();

  const [pipSupported, setPipSupported] = useState(false);
  const [pipActive, setPipActive] = useState(false);

  useEffect(() => {
    let video: HTMLVideoElement | null = null;
    if (isRefObject(videoRef)) {
      video = videoRef.current;
    }
    setPipSupported(
      !!document.pictureInPictureEnabled && pipAction !== undefined,
    );
  }, [videoRef]);

  useEffect(() => {
    let video: HTMLVideoElement | null = null;
    if (isRefObject(videoRef)) {
      video = videoRef.current;
    }
    if (!video) return;
    function onEnter() {
      setPipActive(true);
    }
    function onLeave() {
      setPipActive(false);
    }
    video.addEventListener("enterpictureinpicture", onEnter);
    video.addEventListener("leavepictureinpicture", onLeave);
    return () => {
      if (video) {
        video.removeEventListener("enterpictureinpicture", onEnter);
        video.removeEventListener("leavepictureinpicture", onLeave);
      }
    };
  }, [videoRef]);

  const handlePip = useCallback(() => {
    if (pipAction) pipAction();
  }, [videoRef]);

  const userInteraction = () => {
    setUserInteraction();
  };

  return (
    <View
      position="absolute"
      width="100%"
      height="100%"
      zIndex={999}
      flexDirection="column"
      justifyContent="space-between"
      onPointerMove={userInteraction}
      onTouchStart={userInteraction}
      onPress={onPress}
      {...cursor}
    >
      {muteWasForced && (
        <View
          position="absolute"
          left={0}
          bottom={0}
          padding={20}
          opacity={showControls ? 0 : 1}
        >
          <VolumeX size={60} color="red" />
        </View>
      )}
      {!offline ? null : (
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
          <Text>
            <Offline />
          </Text>
        </View>
      )}
      <Bar
        opacity={showControls ? (fullscreen ? 0 : 1) : 0}
        cursor={embedded ? "pointer" : undefined}
        onPress={() => {
          if (embedded) {
            // Open the current URL in a new window
            const u = new URL(window.location.href);
            u.pathname = u.pathname.replace("/embed", "");
            window.open(u.toString(), "_blank");
            setMuted(true);
          }
        }}
      >
        <Part justifyContent="flex-start" overflow="hidden">
          <View justifyContent="center" paddingLeft="$5" maxWidth="100%">
            <Text wordWrap="break-word" numberOfLines={1} ellipsizeMode="tail">
              @{props.name}
            </Text>
          </View>
        </Part>
        <Part>
          {embedded && m.gtXs ? (
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
          <Viewers viewers={viewers ?? 0} />
        </Part>
      </Bar>
      {isIngesting && <LiveBubble playerId={playerId} />}
      <Bar opacity={showControls ? 1 : 0}>
        <Part justifyContent="flex-start">
          <VolumeSlider showControls={showControls} playerId={playerId} />
        </Part>
        <Part justifyContent="flex-end">
          <PopoverMenu playerId={playerId} />
          {pipSupported && (
            <Pressable
              onPress={handlePip}
              disabled={pipActive}
              style={{
                justifyContent: "center",
                pointerEvents: "auto",
              }}
              accessibilityLabel="Picture in Picture"
            >
              <View paddingLeft="$3" paddingRight="$3" justifyContent="center">
                <svg
                  width="24"
                  height="24"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <rect x="3" y="3" width="18" height="14" rx="2" />
                  <rect x="15" y="13" width="6" height="6" rx="1" />
                </svg>
              </View>
            </Pressable>
          )}
          <Pressable
            style={{
              justifyContent: "center",
            }}
            onPress={() => setFullscreen(!fullscreen)}
          >
            <View paddingLeft="$3" paddingRight="$5" justifyContent="center">
              {fullscreen ? <Minimize /> : <Maximize />}
            </View>
          </Pressable>
        </Part>
      </Bar>
    </View>
  );
}

export function PopoverMenu(props: { playerId?: string }) {
  const playerId = props.playerId;
  const [open, setOpen] = useState(false);
  const media = useMedia();
  const renditions = useRenditions();
  const selectedRendition = usePlayerStore(
    (x) => x.selectedRendition,
    playerId,
  );
  const setSelectedRendition = usePlayerStore(
    (x) => x.setSelectedRendition,
    playerId,
  );
  const protocol = usePlayerStore((x) => x.protocol, playerId);
  const setProtocol = usePlayerStore((x) => x.setProtocol, playerId);
  const showControls = usePlayerStore((x) => x.showControls, playerId);
  const dispatch = useAppDispatch();

  const gearMenu = (
    <GearMenu
      playerId={playerId}
      renditions={renditions}
      selectedRendition={selectedRendition ?? "source"}
      protocol={protocol}
      setSelectedRendition={(rendition) => {
        setSelectedRendition(rendition);
        setOpen(false);
      }}
      setProtocol={(protocol) => {
        setProtocol(intoPlayerProtocol(protocol));
        setOpen(false);
      }}
      dispatch={dispatch}
    />
  );

  useEffect(() => {
    if (!media.sm && showControls === false) {
      setOpen(false);
    }
  }, [showControls, media.sm]);

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

function LiveBubble(props: { playerId?: string }) {
  const playerId = props.playerId;
  const ingestStarting = usePlayerStore((x) => x.ingestStarting, playerId);
  const setIngestStarting = usePlayerStore(
    (x) => x.setIngestStarting,
    playerId,
  );

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
          setIngestStarting(!ingestStarting);
        }}
      >
        <LiveBubbleText playerId={playerId} />
      </Button>
    </View>
  );
}

function LiveBubbleText(props: { playerId?: string }) {
  const playerId = props.playerId;
  const ingestStarting = usePlayerStore((x) => x.ingestStarting, playerId);
  const ingestConnectionState = usePlayerStore(
    (x) => x.ingestConnectionState,
    playerId,
  );

  if (!ingestStarting) {
    return <H3>START STREAMING</H3>;
  }
  console.log("ingest connection state", ingestConnectionState);
  if (ingestConnectionState === "connected") {
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

function GearMenu(props: {
  playerId?: string;
  renditions: PlaceStreamDefs.Rendition[];
  selectedRendition: string;
  protocol: string;
  setSelectedRendition: (rendition: string) => void;
  setProtocol: (protocol: string) => void;
  dispatch: Dispatch<any>;
}) {
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
              onPress={() => setProtocol(PROTOCOL_HLS)}
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
                    selectedRendition === "auto" ? CheckCircle : Circle
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
              iconAfter={selectedRendition === "source" ? CheckCircle : Circle}
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
  const segment = useSegment();
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
