import { useNavigation } from "@react-navigation/native";
import {
  Avatar,
  ContentWarningBadge,
  hexToRgba,
  PlayerStatus,
  PlayerUI,
  ShareSheet,
  Slider,
  Text,
  Toast,
  useAuthor,
  useAvatar,
  useCameraToggle,
  useLivestream,
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
import {
  colors,
  motion,
  scrims,
  statusColors,
  textAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
import { px, py } from "@streamplace/components/src/ui";
import { Image } from "expo-image";
import useAvatars from "hooks/useAvatars";
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
import { Platform, Pressable } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  runOnJS,
  SharedValue,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { MobileChatPanel } from "./chat";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { borders, bottom, gap, h, layout, position, right, w, r } = zero;

export function MobileUi({
  setShowChat,
  showChat,
  hideMobileChat,
  embed = false,
  sharedFadeOpacity,
}: {
  setShowChat?: (show: boolean) => void;
  showChat?: boolean;
  hideMobileChat?: boolean;
  embed?: boolean;
  sharedFadeOpacity?: SharedValue<number>;
}) {
  const { theme } = useTheme();
  const navigation = useNavigation();
  const {
    ingest,
    title,
    setTitle,
    showCountdown,
    setShowCountdown,
    recordSubmitted,
    setRecordSubmitted,
    toggleGoLive,
    toggleStopStream,
    profile: streamProfile,
  } = useLivestreamInfo();
  const { width, height } = usePlayerDimensions();
  const { isPlayerRatioGreater } = useSegmentDimensions();
  const { doSetIngestCamera } = useCameraToggle();
  const avis = useAvatars([streamProfile?.did].filter(Boolean) as string[]);

  const mode = usePlayerStore((state) => state.mode);
  const muteWasForced = usePlayerStore((state) => state.muteWasForced);
  const setMuteWasForced = usePlayerStore((state) => state.setMuteWasForced);
  const [playerIsReady, setPlayerIsReady] = useState(false);
  const playerStatusReady = usePlayerStore(
    (state) => state.status === PlayerStatus.PLAYING,
  );
  useEffect(() => {
    if (playerIsReady) return;
    if (playerStatusReady) {
      setPlayerIsReady(true);
    } else {
      const handle = setTimeout(() => {
        setPlayerIsReady(true);
      }, 5000);
      return () => clearTimeout(handle);
    }
  }, [playerStatusReady]);
  const muted = useMuted();
  const setMuted = useSetMuted();
  const ls = useLivestream();
  const segment = useLivestreamStore((x) => x.segment);
  const contentWarnings =
    (segment?.contentWarnings?.warnings as string[]) || [];

  const { shouldShowFloatingMetrics, shouldShowChatSidePanel, chatPanelWidth } =
    useResponsiveLayout();

  const [showLoading, setShowLoading] = useState(false);

  const openChatOnlyMode = () => {
    const user = streamProfile?.handle;
    if (!user) return;
    navigation.navigate("PopoutChat" as any, { user });
  };

  useEffect(() => {
    if (recordSubmitted) setShowLoading(false);
  }, [recordSubmitted]);

  const isPortrait = !shouldShowChatSidePanel;
  const isSelfAndNotLive = ingest !== null && ls === null;
  const isSelfAndLive = ingest !== null && ls !== null;

  // Controls auto-hide after 3s of inactivity (design-system standard)
  const FADE_OUT_DELAY = 3000;
  const internalFadeOpacity = useSharedValue(1);
  const fadeOpacity = sharedFadeOpacity ?? internalFadeOpacity;
  const fadeTimeout = useRef<NodeJS.Timeout | null>(null);
  const selectedRendition = usePlayerStore((state) => state.selectedRendition);

  const resetFadeTimer = () => {
    fadeOpacity.value = withTiming(1, { duration: motion.base });
    if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    if (selectedRendition === "audio") return;
    if (ingest !== null) return;
    fadeTimeout.current = setTimeout(() => {
      fadeOpacity.value = withTiming(0, { duration: motion.slow });
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
    fadeOpacity.value = withTiming(1, { duration: motion.base });
  };

  const onPlayerHover = () => {
    "worklet";
    showUI();
    // Schedule timer reset on JS thread
    runOnJS(resetFadeTimer)();
  };

  const animatedFadeStyle = useAnimatedStyle(() => ({
    opacity: fadeOpacity.value,
    transform: isPortrait ? [{ translateY: (fadeOpacity.value - 1) * 40 }] : [],
  }));

  const hover = Gesture.Hover().onChange(onPlayerHover);
  const pan = Gesture.Pan().onChange(onPlayerHover);
  const tap = Gesture.Tap().onEnd(onPlayerHover);
  let chatSection: React.ReactNode = null;
  if (
    mode !== "vod" &&
    !hideMobileChat &&
    !isSelfAndNotLive &&
    playerIsReady &&
    isPortrait
  ) {
    chatSection = (
      <MobileChatPanel isPlayerRatioGreater={isPlayerRatioGreater} />
    );
  }
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
              {ingest === null ? (
                <SafeAreaView
                  // VOD's container already insets the video below the notch, so
                  // the controls only need the small style padding; live fills
                  // the window and clears the notch with the top edge inset.
                  edges={mode === "vod" ? [] : ["top"]}
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    py[2],
                    gap.all[4],
                    w.percent[100],
                  ]}
                >
                  <Pressable
                    onPress={() => {
                      navigation.canGoBack()
                        ? navigation.goBack()
                        : navigation.navigate("MainTabs" as any, {
                            screen: "HomeTab",
                          });
                    }}
                    style={[
                      {
                        padding: 9,
                        backgroundColor: scrims.light,
                        borderRadius: 12,
                      },
                      r[2],
                    ]}
                  >
                    <ChevronLeft color={colors.white} />
                  </Pressable>
                  {/* if we're in landscape mode show the profile picture and username */}
                  {shouldShowChatSidePanel && (
                    <View
                      style={[
                        {
                          backgroundColor: scrims.light,
                          borderRadius: 12,
                        },
                        r[2],
                        layout.flex.row,
                        layout.flex.alignCenter,
                        gap.all[1],
                        px[2],
                        py[1],
                      ]}
                    >
                      <Avatar
                        src={
                          streamProfile?.did && avis[streamProfile?.did].avatar
                        }
                        name={streamProfile?.handle}
                        size="sm"
                      />
                      <Text
                        numberOfLines={1}
                        ellipsizeMode="tail"
                        style={{
                          color: colors.white,
                          fontSize: 14,
                          marginLeft: 8,
                        }}
                      >
                        {streamProfile?.handle}
                      </Text>
                    </View>
                  )}
                  <ContentWarningBadge warnings={contentWarnings} truncate />
                  <View style={{ flex: 1 }} />
                  <ShareSheet />
                  <PlayerUI.ContextMenu
                    onOpenChat={
                      streamProfile?.handle ? openChatOnlyMode : undefined
                    }
                  />
                  {shouldShowChatSidePanel && setShowChat && (
                    <Pressable
                      onPress={() => {
                        setShowChat(!showChat);
                      }}
                    >
                      {showChat ? (
                        <ChevronRight color={colors.white} size={20} />
                      ) : (
                        <ChevronLeft color={colors.white} size={20} />
                      )}
                    </Pressable>
                  )}
                </SafeAreaView>
              ) : (
                <SafeAreaView
                  // VOD's container already insets the video below the notch, so
                  // the controls only need the small style padding; live fills
                  // the window and clears the notch with the top edge inset.
                  edges={mode === "vod" ? [] : ["top"]}
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
                    style={[
                      layout.flex.column,
                      gap.all[2],
                      { maxWidth: "70%" },
                    ]}
                  >
                    <LeftControlsPanel
                      navigation={navigation}
                      muted={muted}
                      setMuted={setMuted}
                      muteWasForced={muteWasForced}
                      setMuteWasForced={setMuteWasForced}
                    />
                  </View>

                  {/* Right Controls Column */}
                  <View
                    style={[
                      layout.flex.row,
                      gap.all[2],
                      layout.flex.align.start,
                    ]}
                  >
                    <View>
                      <View
                        style={[
                          {
                            padding: 9,
                            backgroundColor: scrims.light,
                            borderRadius: 12,
                          },
                          r[2],
                        ]}
                      >
                        <ShareSheet />
                      </View>
                    </View>

                    <RightControlsPanel
                      ingest={ingest}
                      doSetIngestCamera={doSetIngestCamera}
                      shouldShowChatSidePanel={shouldShowChatSidePanel}
                      isPortrait={!shouldShowChatSidePanel}
                      showChat={showChat}
                      setShowChat={setShowChat}
                    />
                  </View>
                </SafeAreaView>
              )}

              {shouldShowFloatingMetrics && isSelfAndLive && (
                <View
                  style={[
                    layout.position.absolute,
                    position.top[32],
                    position.left[0],
                    position.right[0],
                    layout.flex.column,
                    layout.flex.center,
                  ]}
                >
                  <PlayerUI.MetricsPanel
                    showMetrics={shouldShowFloatingMetrics}
                  />
                </View>
              )}
              {Platform.OS !== "web" && (
                <View
                  style={[layout.position.absolute, { bottom: 12, right: 12 }]}
                >
                  <RotateButton />
                </View>
              )}
            </View>
            {isSelfAndNotLive && (
              <PlayerUI.InputPanel
                title={title}
                setTitle={setTitle}
                toggleGoLive={toggleGoLive}
                isLive={isSelfAndLive}
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
      {/* VOD scrub/play controls live OUTSIDE the gesture detector so the seek
          bar's own pan gesture isn't swallowed by the overlay's tap/pan Race.
          They still fade with the rest of the UI via the shared opacity. */}
      {mode === "vod" && (
        <Animated.View
          style={[
            { position: "absolute", bottom: 8, left: 0, right: 0 },
            animatedFadeStyle,
          ]}
          // Let taps on empty space fall through to the overlay's tap-to-reveal
          // gesture; only the controls themselves capture touches.
          pointerEvents="box-none"
        >
          <PlayerUI.SeekBar />
          <PlayerUI.VodControls />
        </Animated.View>
      )}
      {chatSection}
    </>
  );
}

function LeftControlsPanel({
  navigation,
  muted,
  setMuted,
  muteWasForced,
  setMuteWasForced,
}: {
  navigation: any;
  muted: boolean;
  setMuted: (muted: boolean) => void;
  muteWasForced: boolean;
  setMuteWasForced: (forced: boolean) => void;
}) {
  const profile = useAuthor();
  const avatar = useAvatar();
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
            backgroundColor: scrims.light,
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
                : navigation.navigate("MainTabs" as any, { screen: "HomeTab" });
            }}
          >
            <ChevronLeft color={colors.white} />
          </Pressable>
          <Image
            source={
              avatar ? { uri: avatar } : require("assets/images/goose.png")
            }
            key={profile?.did}
            style={[
              {
                width: 36,
                height: 36,
              },
              { borderRadius: 999 },
              borders.width.thin,
              borders.color.gray[700],
            ]}
          />
          <Text numberOfLines={1} ellipsizeMode="tail">
            {profile?.handle}
          </Text>
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
                backgroundColor: scrims.light,
                borderRadius: 999,
                borderWidth: 2,
                borderColor: hexToRgba(statusColors.dark.danger, 0.2),
              },
            ]}
          >
            <VolumeX
              size="24"
              color={hexToRgba(statusColors.dark.danger, 0.8)}
            />
          </View>
          <Text color="muted" size="sm">
            Tap to unmute
          </Text>
        </Pressable>
      )}
      <View>
        <ContentWarningBadge warnings={contentWarnings} truncate />
      </View>
    </>
  );
}

function RightControlsPanel({
  ingest,
  doSetIngestCamera,
  shouldShowChatSidePanel,
  isPortrait,
  showChat,
  setShowChat,
}: {
  ingest: string | null;
  doSetIngestCamera: () => void;
  shouldShowChatSidePanel: boolean;
  isPortrait: boolean;
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
            backgroundColor: scrims.light,
            borderRadius: 12,
            paddingVertical: 2.25 * 4,
          },
          zero.r[2],
          isPortrait ? zero.layout.flex.column : zero.layout.flex.row,
          zero.layout.flex.center,
          zero.gap.all[4],
          isPortrait ? zero.px[2] : zero.px[3],
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
            {Platform.OS === "web" && <PlayerUI.StreamContextMenu />}
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
        ) : ingest === null ? (
          <PlayerUI.ContextMenu />
        ) : (
          <PlayerUI.StreamContextMenu />
        )}
        {shouldShowChatSidePanel && setShowChat && (
          <Pressable
            onPress={() => {
              setShowChat(!showChat);
            }}
          >
            {showChat ? (
              <ChevronRight color={colors.white} size={20} />
            ) : (
              <ChevronLeft color={colors.white} size={20} />
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
              backgroundColor: scrims.dark,
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
                backgroundColor: textAlphas.dark[4],
                borderRadius: 999,
                top: "50%",
                transform: [{ translateY: -1.5 }],
              }}
            >
              <Slider.Range
                style={{
                  position: "absolute",
                  backgroundColor: colors.white,
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
                  backgroundColor: colors.white,
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

/** Native fullscreen/rotation toggle, pinned to the player's bottom-right. */
function RotateButton() {
  const { theme } = useTheme();
  const { toggleRotation, canRotate, currentOrientation } = useRotation();
  if (!canRotate) return null;
  return (
    <View
      style={[
        { padding: 9, backgroundColor: scrims.light, borderRadius: 12 },
        r[2],
      ]}
    >
      <Pressable onPress={toggleRotation}>
        {currentOrientation === 1 ? (
          <Maximize color={theme.colors.foreground} size={20} />
        ) : (
          <Minimize color={theme.colors.foreground} size={20} />
        )}
      </Pressable>
    </View>
  );
}
