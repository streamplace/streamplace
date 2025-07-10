import { useNavigation } from "@react-navigation/native";
import {
  PlayerUI,
  Slider,
  Text,
  Toast,
  useAvatars,
  useCameraToggle,
  useLivestreamInfo,
  usePlayerDimensions,
  usePlayerStore,
  View,
  zero,
} from "@streamplace/components";
import {
  ChevronLeft,
  Fullscreen,
  MessageSquare,
  Minimize,
  SwitchCamera,
  Volume2,
  VolumeX,
} from "lucide-react-native";
import { useCallback, useEffect, useRef, useState } from "react";
import { Image, Platform, Pressable } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { borders, colors, gap, h, layout, position, w, px, py, r, p, bg, text } =
  zero;

// Live indicator bubble component
function LiveBubble() {
  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[1],
        px[2],
        py[1],
        r[2],
        bg.destructive[500],
        borders.width.thin,
        borders.color.gray[800],
      ]}
    >
      <View style={[h[2], w[2], bg.white]} />
      <Text
        style={[
          text.white,
          { fontSize: 12, lineHeight: 16, fontWeight: "600" },
        ]}
      >
        LIVE
      </Text>
    </View>
  );
}

// Volume slider component
function VolumeSlider() {
  const muted = usePlayerStore((state) => state.muted);
  const setMuted = usePlayerStore((state) => state.setMuted);
  const volume = usePlayerStore((state) => state.volume);
  const setVolume = usePlayerStore((state) => state.setVolume);

  const fadeAnim = useSharedValue(0);
  const widthAnim = useSharedValue(0);

  const onVolumeHover = useCallback(() => {
    fadeAnim.value = withTiming(1, { duration: 200 });
    widthAnim.value = withTiming(200, { duration: 200 });
  }, [fadeAnim, widthAnim]);

  // Toggle mute state
  const handleMuteToggle = useCallback(() => {
    setMuted(!muted);
  }, [muted, setMuted]);

  const VolumeIcon = muted ? VolumeX : Volume2;

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: fadeAnim.value,
    width: widthAnim.value,
  }));

  // Convert volume (0-1) to percentage (0-100) for slider
  const sliderValue = (muted ? 0 : volume) * 100;
  return (
    <View
      onPointerEnter={onVolumeHover}
      style={[layout.flex.row, layout.flex.alignCenter, { height: 50 }]}
    >
      <Pressable onPress={handleMuteToggle} style={[p[2], r[1]]}>
        <VolumeIcon size={20} color="white" />
      </Pressable>

      <Animated.View style={[{ height: 30 }, animatedStyle]}>
        <Slider.Root
          style={{
            position: "relative",
            display: "flex",
            alignItems: "center",
            flex: 1,
            width: 200,
            height: 20,
          }}
          value={sliderValue}
          min={0}
          max={100} // Slider max value is 100 for percentage
          onValueChange={(vals) => {
            const newVolume = vals[0] / 100; // Convert back to 0-1 range
            setVolume(newVolume);
            if (newVolume === 0) {
              setMuted(true);
            } else {
              setMuted(false);
            }
          }}
          asChild
        >
          <Slider.Track
            style={{
              flexGrow: 1,
              height: 30,
              position: "relative",
              flex: 1,
            }}
          >
            <Slider.Range
              style={{
                position: "absolute",
                backgroundColor: "white",
                borderRadius: 999,
                height: 3,
                flex: 1,
                width: "100%",
                transform: [{ translateY: 14 }],
              }}
            />
            <Slider.Thumb
              style={{
                position: "absolute",
                width: 16,
                height: 16,
                borderRadius: 8,
                backgroundColor: "white",
                boxShadow: "0 2px 10px rgba(0, 0, 0, 0.2)",
                transform: [{ translateX: -8 }, { translateY: 7 }],
              }}
            />
          </Slider.Track>
        </Slider.Root>
      </Animated.View>
    </View>
  );
}

export function DesktopUi() {
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
  const { doSetIngestCamera } = useCameraToggle();
  const avatars = useAvatars(profile?.did ? [profile?.did] : []);
  const { safeAreaInsets, shouldShowFloatingMetrics } = useResponsiveLayout();

  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const setFullscreen = usePlayerStore((state) => state.setFullscreen);
  const muteWasForced = usePlayerStore((state) => state.muteWasForced);
  const setMuteWasForced = usePlayerStore((state) => state.setMuteWasForced);
  const setMuted = usePlayerStore((state) => state.setMuted);

  const [isControlsVisible, setIsControlsVisible] = useState(true);
  const [isChatOpen, setIsChatOpen] = useState(false);
  const fadeOpacity = useSharedValue(1);
  const fadeTimeout = useRef<NodeJS.Timeout | null>(null);
  const FADE_OUT_DELAY = 500;

  const isSelfAndNotLive = ingest === "new";
  const isActivelyLive = ingest !== null && ingest !== "new";

  const resetFadeTimer = useCallback(() => {
    fadeOpacity.value = withTiming(1, { duration: 200 });
    if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    setIsControlsVisible(true);

    fadeTimeout.current = setTimeout(() => {
      fadeOpacity.value = withTiming(0, { duration: 400 });
      setIsControlsVisible(false);
    }, FADE_OUT_DELAY);
  }, [fadeOpacity]);

  const onPlayerHover = useCallback(() => {
    resetFadeTimer();
  }, [resetFadeTimer]);

  const toggleChat = useCallback(() => {
    setIsChatOpen((prev) => !prev);
  }, []);

  useEffect(() => {
    resetFadeTimer();

    return () => {
      if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
      if (ingestStarting) {
        setIngestStarting(false);
      }
    };
  }, [ingestStarting, setIngestStarting, resetFadeTimer]);

  const animatedFadeStyle = useAnimatedStyle(() => ({
    opacity: shouldShowFloatingMetrics ? 1 : fadeOpacity.value,
  }));

  const hover = Gesture.Hover().onChange((_) => runOnJS(onPlayerHover)());

  return (
    <GestureDetector gesture={hover}>
      <>
        <View
          style={[layout.position.absolute, h.percent[100], w.percent[100]]}
        >
          <Animated.View
            style={[
              layout.position.absolute,
              w.percent[100],
              {
                top: safeAreaInsets.top,
                paddingHorizontal: 16,
                paddingVertical: 16,
              },
              animatedFadeStyle,
            ]}
          >
            <View
              style={[
                layout.flex.row,
                layout.flex.spaceBetween,
                layout.flex.alignCenter,
              ]}
            >
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}
              >
                {Platform.OS !== "web" && (
                  <Pressable
                    onPress={() => {
                      navigation.canGoBack()
                        ? navigation.goBack()
                        : navigation.navigate("Home", { screen: "StreamList" });
                    }}
                    style={[p[2], r[1]]}
                  >
                    <ChevronLeft color="white" size={24} />
                  </Pressable>
                )}
                <Image
                  source={
                    profile?.did
                      ? { uri: avatars[profile?.did]?.avatar }
                      : require("assets/images/goose.png")
                  }
                  style={[
                    {
                      width: 40,
                      height: 40,
                      borderRadius: 20,
                      backgroundColor: colors.gray[800],
                    },
                    borders.width.thin,
                    borders.color.gray[700],
                  ]}
                />

                <View style={[layout.flex.column, gap.all[1]]}>
                  <Text
                    style={[text.white, { fontSize: 16, fontWeight: "600" }]}
                  >
                    {profile?.handle}
                  </Text>
                  {isActivelyLive && <LiveBubble />}
                </View>
              </View>

              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}
              >
                {isActivelyLive && (
                  <>
                    <PlayerUI.Viewers />

                    <Pressable onPress={toggleChat} style={[p[2], r[1]]}>
                      <MessageSquare
                        size={20}
                        color={isChatOpen ? colors.primary[500] : colors.white}
                      />
                    </Pressable>
                  </>
                )}
                {ingest !== null && (
                  <Pressable onPress={doSetIngestCamera} style={[p[2], r[1]]}>
                    <SwitchCamera size={24} color={colors.gray[200]} />
                  </Pressable>
                )}
              </View>
            </View>
          </Animated.View>

          {isActivelyLive && isControlsVisible && (
            <View
              style={[
                layout.position.absolute,
                {
                  transform: [{ translateX: -100 }, { translateY: -25 }],
                },
              ]}
            >
              <Animated.View
                style={[
                  {
                    padding: 12,
                    backgroundColor: "rgba(0, 0, 0, 0.5)",
                  },
                  r[3],
                  animatedFadeStyle,
                ]}
              >
                <PlayerUI.MetricsPanel showMetrics={isActivelyLive} />
              </Animated.View>
            </View>
          )}

          <Animated.View
            style={[
              layout.position.absolute,
              position.bottom[0],
              w.percent[100],
              {
                backgroundColor: "rgba(0, 0, 0, 0.6)",
                paddingHorizontal: 16,
                paddingVertical: 2,
                paddingBottom: 2,
              },
              animatedFadeStyle,
            ]}
          >
            <View
              style={[
                layout.flex.row,
                layout.flex.spaceBetween,
                layout.flex.alignCenter,
              ]}
            >
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[4]]}
              >
                <VolumeSlider />
              </View>

              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}
              >
                {Platform.OS === "web" && (
                  <Pressable
                    onPress={() => {
                      setFullscreen(!fullscreen);
                    }}
                    style={[p[2], r[1]]}
                  >
                    {fullscreen ? <Minimize /> : <Fullscreen />}
                  </Pressable>
                )}
                {ingest === null && <PlayerUI.ContextMenu />}
              </View>
            </View>
          </Animated.View>

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
            height={height}
            onDone={() => {
              setShowCountdown(false);
            }}
          />

          <Toast
            open={recordSubmitted}
            onOpenChange={setRecordSubmitted}
            title="You're live!"
            description="We're notifying your followers that you just went live."
            duration={5}
          />
          {muteWasForced && (
            <View
              style={[
                layout.position.absolute,
                layout.flex.center,
                h.percent[100],
                w.percent[100],
              ]}
            >
              <Pressable
                onPress={() => {
                  if (muteWasForced) {
                    setMuted(false);
                    setMuteWasForced(false);
                  }
                }}
                style={[
                  p[4],
                  {
                    backgroundColor: "rgba(50, 30, 30, 0.7)",
                    borderRadius: 999,
                    borderWidth: 2,
                    borderColor: colors.gray[300],
                  },
                ]}
              >
                <VolumeX size="48" color="rgba(255,200,200)" />
              </Pressable>
            </View>
          )}
        </View>
      </>
    </GestureDetector>
  );
}
