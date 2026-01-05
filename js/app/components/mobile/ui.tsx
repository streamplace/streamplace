import { useNavigation } from "@react-navigation/native";
import {
  ContentWarningBadge,
  PlayerUI,
  Slider,
  Text,
  Toast,
  useAvatars,
  useCameraToggle,
  useLivestreamInfo,
  useLivestreamStore,
  useMuted,
  usePlayerDimensions,
  usePlayerStore,
  useRotation,
  useSegmentDimensions,
  useSetMuted,
  useSetVolume,
  useTheme,
  useVolume,
  View,
  zero,
} from "@streamplace/components";
import { px, py } from "@streamplace/components/src/ui";
import {
  ChevronLeft,
  ChevronRight,
  Fullscreen,
  Maximize,
  Minimize,
  SwitchCamera,
  Volume2,
  VolumeX,
} from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import { Image, Platform, Pressable } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { MobileChatPanel } from "./chat";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { borders, gap, h, layout, position, w, r } = zero;

export function MobileUi({
  setShowChat,
  showChat,
  hideMobileChat,
  embed = false,
}: {
  setShowChat?: (show: boolean) => void;
  showChat?: boolean;
  hideMobileChat?: boolean;
  embed?: boolean;
}) {
  const { theme } = useTheme();
  const navigation = useNavigation();
  const {
    ingest,
    profile,
    title,
    setTitle,
    showCountdown,
    setShowCountdown,
    recordSubmitted,
    setRecordSubmitted,
    ingestStarting,
    setIngestStarting,
    toggleGoLive,
  } = useLivestreamInfo();
  const { width, height } = usePlayerDimensions();
  const { isPlayerRatioGreater } = useSegmentDimensions();
  const { doSetIngestCamera } = useCameraToggle();
  const avatars = useAvatars(profile?.did ? [profile?.did] : []);

  const muteWasForced = usePlayerStore((state) => state.muteWasForced);
  const setMuteWasForced = usePlayerStore((state) => state.setMuteWasForced);
  const muted = useMuted();
  const setMuted = useSetMuted();

  const { shouldShowFloatingMetrics, shouldShowChatSidePanel, chatPanelWidth } =
    useResponsiveLayout();

  const [showLoading, setShowLoading] = useState(false);

  useEffect(() => {
    return () => {
      if (ingestStarting) {
        setIngestStarting(false);
      }
    };
  }, [ingestStarting, setIngestStarting]);

  useEffect(() => {
    if (recordSubmitted) setShowLoading(false);
  }, [recordSubmitted]);

  const isSelfAndNotLive = ingest === "new";
  const isLive = ingest !== null && ingest !== "new";

  const FADE_OUT_DELAY = 4000;
  const fadeOpacity = useSharedValue(1);
  const fadeTimeout = useRef<NodeJS.Timeout | null>(null);

  const resetFadeTimer = () => {
    fadeOpacity.value = withTiming(1, { duration: 200 });
    if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    fadeTimeout.current = setTimeout(() => {
      fadeOpacity.value = withTiming(0, { duration: 400 });
    }, FADE_OUT_DELAY);
  };

  useEffect(() => {
    resetFadeTimer();
    return () => {
      if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    };
  }, []);

  const showUI = () => {
    "worklet";
    fadeOpacity.value = withTiming(1, { duration: 200 });
  };

  const onPlayerHover = () => {
    "worklet";
    showUI();
    // Schedule timer reset on JS thread
    runOnJS(resetFadeTimer)();
  };

  const animatedFadeStyle = useAnimatedStyle(() => ({
    opacity:
      shouldShowFloatingMetrics || showChat === undefined
        ? 1
        : fadeOpacity.value,
  }));

  const hover = Gesture.Hover().onChange(onPlayerHover);
  const pan = Gesture.Pan().onChange(onPlayerHover);
  const tap = Gesture.Tap().onEnd(onPlayerHover);

  const combined = Gesture.Race(hover, pan, tap);
  return (
    <>
      <GestureDetector gesture={combined}>
        <View
          style={[layout.position.absolute, h.percent[100], w.percent[100]]}
        >
          <Animated.View
            style={[
              layout.position.absolute,
              h.percent[100],
              w.percent[100],
              animatedFadeStyle,
            ]}
          >
            {/* Main UI Overlay */}
            <View style={[h.percent[100], w.percent[100]]}>
              <SafeAreaView
                edges={["top"]}
                style={[
                  px[2],
                  py[2],
                  layout.flex.row,
                  layout.flex.spaceBetween,
                  w.percent[100],
                ]}
              >
                {/* Left Controls Column */}
                <View
                  style={[layout.flex.column, gap.all[2], { maxWidth: "70%" }]}
                >
                  <LeftControlsPanel
                    navigation={navigation}
                    profile={profile}
                    avatars={avatars}
                    muted={muted}
                    setMuted={setMuted}
                    muteWasForced={muteWasForced}
                    setMuteWasForced={setMuteWasForced}
                  />
                </View>

                {/* Right Controls Column */}
                <View
                  style={[layout.flex.row, gap.all[2], layout.flex.align.start]}
                >
                  {shouldShowFloatingMetrics && (
                    <View>
                      <View
                        style={[
                          {
                            padding: 9,
                            backgroundColor: "rgba(90,90,90, 0.3)",
                            borderRadius: 12,
                          },
                          r[2],
                        ]}
                      >
                        <PlayerUI.Viewers />
                      </View>
                    </View>
                  )}

                  <RightControlsPanel
                    ingest={ingest}
                    doSetIngestCamera={doSetIngestCamera}
                    shouldShowChatSidePanel={shouldShowChatSidePanel}
                    showChat={showChat}
                    setShowChat={setShowChat}
                  />
                </View>
              </SafeAreaView>

              {shouldShowFloatingMetrics && isLive && (
                <View
                  style={[
                    layout.position.absolute,
                    position.top[28],
                    position.left[0],
                    position.right[0],
                    layout.flex.column,
                    layout.flex.center,
                  ]}
                >
                  <PlayerUI.MetricsPanel
                    showMetrics={isLive || isSelfAndNotLive}
                  />
                </View>
              )}
            </View>
            {isSelfAndNotLive && (
              <PlayerUI.InputPanel
                title={title}
                setTitle={setTitle}
                ingestStarting={ingestStarting}
                toggleGoLive={toggleGoLive}
              />
            )}

            <PlayerUI.CountdownOverlay
              visible={showCountdown}
              width={width}
              height={height - 150}
              onDone={() => {
                if (!recordSubmitted && title != "") {
                  setShowLoading(true);
                }
                setShowCountdown(false);
              }}
            />
            <PlayerUI.LoadingOverlay
              visible={showLoading}
              width={width}
              height={height - 150}
              subtitle="We're setting up your stream."
            />

            <Toast
              open={recordSubmitted}
              onOpenChange={setRecordSubmitted}
              title="You're live!"
              description="We're notifying your followers that you just went live."
              duration={5}
            />
          </Animated.View>
          <PlayerUI.AutoplayButton />
        </View>
      </GestureDetector>
      {showChat === undefined && ingest !== "new" && (
        <MobileChatPanel isPlayerRatioGreater={isPlayerRatioGreater} />
      )}
    </>
  );
}

function LeftControlsPanel({
  navigation,
  profile,
  avatars,
  muted,
  setMuted,
  muteWasForced,
  setMuteWasForced,
}: {
  navigation: any;
  profile: any;
  avatars: any;
  muted: boolean;
  setMuted: (muted: boolean) => void;
  muteWasForced: boolean;
  setMuteWasForced: (forced: boolean) => void;
}) {
  // Get content warnings from segment
  const segment = useLivestreamStore((x) => x.segment);
  const contentWarnings =
    (segment?.contentWarnings?.warnings as string[]) || [];

  return (
    <>
      {/* Back Button and Profile */}
      <View
        style={[
          {
            padding: 3,
            paddingRight: 8,
            backgroundColor: "rgba(90,90,90, 0.25)",
            borderRadius: 12,
            alignSelf: "flex-start",
          },
          r[2],
        ]}
      >
        <View style={[layout.flex.row, layout.flex.center, gap.all[2]]}>
          <Pressable
            onPress={() => {
              navigation.canGoBack()
                ? navigation.goBack()
                : navigation.navigate("Home", {
                    screen: "StreamList",
                  });
            }}
          >
            <ChevronLeft color="white" />
          </Pressable>
          <Image
            source={
              profile?.did
                ? { url: avatars[profile?.did]?.avatar }
                : require("assets/images/goose.png")
            }
            width={32}
            height={32}
            style={[
              {
                width: 36,
                height: 36,
                backgroundColor: "green",
              },
              { borderRadius: 999 },
              borders.width.thin,
              borders.color.gray[700],
            ]}
          />
          <Text>{profile?.handle}</Text>
        </View>
      </View>

      {/* Muted indicator */}
      {muted && (
        <Pressable
          onPress={() => {
            if (muteWasForced) {
              setMuted(false);
              setMuteWasForced(false);
            } else {
              setMuted(false);
            }
          }}
          style={[
            {
              flexDirection: "row",
              alignItems: "center",
              gap: 8,
            },
          ]}
        >
          <View
            style={[
              {
                padding: 4,
                backgroundColor: "rgba(50, 30, 30, 0.4)",
                borderRadius: 999,
                borderWidth: 2,
                borderColor: "rgba(255, 120, 120, 0.2)",
              },
            ]}
          >
            <VolumeX size="24" color="rgba(255,120,120,0.8)" />
          </View>
          <Text color="muted" size="sm">
            Tap to unmute
          </Text>
        </Pressable>
      )}
      <View>
        <ContentWarningBadge warnings={contentWarnings} />
      </View>
    </>
  );
}

function RightControlsPanel({
  ingest,
  doSetIngestCamera,
  shouldShowChatSidePanel,
  showChat,
  setShowChat,
}: {
  ingest: string | null;
  doSetIngestCamera: () => void;
  shouldShowChatSidePanel: boolean;
  showChat?: boolean;
  setShowChat?: (show: boolean) => void;
}) {
  const { theme } = useTheme();
  const volume = useVolume();
  const setVolume = useSetVolume();
  const muted = useMuted();
  const setMuted = useSetMuted();
  const fullscreen = usePlayerStore((x) => x.fullscreen);
  const setFullscreen = usePlayerStore((x) => x.setFullscreen);
  const { toggleRotation, canRotate, currentOrientation } = useRotation();

  const [showVolumeSlider, setShowVolumeSlider] = useState(false);

  const handleVolumePress = () => {
    setShowVolumeSlider(!showVolumeSlider);
  };

  const handleVolumeChange = (values: number[]) => {
    const newVolume = values[0] / 100; // Convert from 0-100 to 0-1
    setVolume(newVolume);
    if (newVolume === 0) {
      setMuted(true);
    } else {
      setMuted(false);
    }
  };

  const sliderValue = (muted ? 0 : volume) * 100;

  return (
    <View
      style={[
        zero.layout.flex.column,
        zero.gap.all[2],
        zero.layout.flex.align.end,
      ]}
    >
      <View
        style={[
          {
            backgroundColor: "rgba(90,90,90, 0.3)",
            borderRadius: 12,
            paddingVertical: 2.25 * 4,
          },
          zero.r[2],
          showChat === undefined
            ? zero.layout.flex.column
            : zero.layout.flex.row,
          zero.layout.flex.center,
          zero.gap.all[4],
          showChat === undefined ? zero.px[2] : zero.px[3],
          zero.layout.position.relative,
        ]}
      >
        {ingest === null ? (
          Platform.OS === "web" && <PlayerUI.ContextMenu />
        ) : (
          <>
            <Pressable onPress={doSetIngestCamera}>
              <SwitchCamera color={theme.colors.foreground} size={20} />
            </Pressable>
          </>
        )}
        {Platform.OS === "web" ? (
          <>
            <Pressable
              onPress={() => {
                setFullscreen(!fullscreen);
              }}
              style={[zero.p[2], r[1]]}
            >
              {fullscreen ? (
                <Minimize color={theme.colors.text} size={20} />
              ) : (
                <Fullscreen color={theme.colors.text} size={20} />
              )}
            </Pressable>
            <Pressable onPress={handleVolumePress}>
              {muted || volume === 0 ? (
                <VolumeX color={theme.colors.foreground} size={20} />
              ) : (
                <Volume2 color={theme.colors.foreground} size={20} />
              )}
            </Pressable>
          </>
        ) : (
          <View
            style={[
              showChat === undefined
                ? { flexDirection: "column-reverse" }
                : { flexDirection: "row" },
              zero.layout.flex.center,
              zero.gap.all[4],
            ]}
          >
            {canRotate && (
              <Pressable
                onPress={() => {
                  toggleRotation();
                }}
              >
                {currentOrientation === 1 ? (
                  <Maximize color={theme.colors.foreground} size={20} />
                ) : (
                  <Minimize color={theme.colors.foreground} size={20} />
                )}
              </Pressable>
            )}
            <PlayerUI.ContextMenu />
          </View>
        )}
        {shouldShowChatSidePanel && setShowChat && (
          <Pressable
            onPress={() => {
              setShowChat(!showChat);
            }}
          >
            {showChat ? (
              <ChevronRight color="white" size={20} />
            ) : (
              <ChevronLeft color="white" size={20} />
            )}
          </Pressable>
        )}
      </View>
      {/* Volume Slider Popup */}
      {showVolumeSlider && (
        <View
          style={[
            {
              padding: 10,
              backgroundColor: "rgba(90,90,90, 0.9)",
              borderRadius: 12,
              width: 150,
              height: 36,
              bottom: -36 - 10,
            },
            zero.r[2],
            zero.layout.position.absolute,
          ]}
          onTouchStart={(e) => e.stopPropagation()}
          onTouchMove={(e) => e.stopPropagation()}
          onTouchEnd={(e) => e.stopPropagation()}
        >
          <Slider.Root
            style={{
              position: "relative",
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: 12,
            }}
            value={sliderValue}
            min={0}
            max={100}
            onValueChange={handleVolumeChange}
          >
            <Slider.Track
              style={{
                position: "absolute",
                width: "100%",
                height: 3,
                backgroundColor: "rgba(255,255,255,0.3)",
                borderRadius: 999,
                top: "50%",
                transform: [{ translateY: -1.5 }],
              }}
            >
              <Slider.Range
                style={{
                  position: "absolute",
                  backgroundColor: "white",
                  borderRadius: 999,
                  height: 3,
                  top: 0,
                }}
              />
              <Slider.Thumb
                style={{
                  position: "absolute",
                  width: 16,
                  height: 16,
                  borderRadius: 8,
                  backgroundColor: "white",
                  top: -6.5,
                  transform: [{ translateX: -8 }],
                }}
              />
            </Slider.Track>
          </Slider.Root>
        </View>
      )}
    </View>
  );
}
