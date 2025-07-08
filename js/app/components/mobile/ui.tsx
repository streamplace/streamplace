import { useNavigation } from "@react-navigation/native";
import {
  PlayerUI,
  Text,
  Toast,
  useAvatars,
  useCameraToggle,
  useLivestreamInfo,
  usePlayerDimensions,
  useSegmentDimensions,
  View,
  zero,
} from "@streamplace/components";
import { ChevronLeft, SwitchCamera } from "lucide-react-native";
import { useEffect, useRef } from "react";
import { Image, Pressable, TouchableWithoutFeedback } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { MobileChatPanel } from "./chat";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { borders, colors, gap, h, layout, position, w, bottom, px, py, r } =
  zero;

export function MobileUi() {
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

  const { shouldShowFloatingMetrics, safeAreaInsets } = useResponsiveLayout();

  useEffect(() => {
    return () => {
      if (ingestStarting) {
        setIngestStarting(false);
      }
    };
  }, [ingestStarting, setIngestStarting]);

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

  const animatedFadeStyle = useAnimatedStyle(() => ({
    opacity: shouldShowFloatingMetrics ? 1 : fadeOpacity.value,
  }));

  return (
    <>
      <TouchableWithoutFeedback onPress={resetFadeTimer}>
        <Animated.View
          style={[
            layout.position.absolute,
            h.percent[100],
            w.percent[100],
            animatedFadeStyle,
          ]}
        >
          {/* Main UI Overlay */}
          <View
            style={[layout.position.absolute, h.percent[100], w.percent[100]]}
          >
            {/* Top Left - Back Button and Profile */}
            <View
              style={[
                {
                  padding: 6.5,
                  backgroundColor: "rgba(0, 0, 0, 0.6)",
                },
                r[2],
                layout.position.absolute,
                position.left[1],
                { top: 8 },
              ]}
            >
              <View style={[layout.flex.row, layout.flex.center, gap.all[2]]}>
                <Pressable
                  onPress={() => {
                    navigation.canGoBack()
                      ? navigation.goBack()
                      : navigation.navigate("Home", { screen: "StreamList" });
                  }}
                >
                  <ChevronLeft color="white" />
                </Pressable>
                {shouldShowFloatingMetrics && (
                  <>
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
                  </>
                )}
              </View>
            </View>

            {shouldShowFloatingMetrics && (
              <View
                style={[
                  {
                    padding: 10,
                    backgroundColor: "rgba(0, 0, 0, 0.5)",
                  },
                  r[2],
                  layout.position.absolute,
                  position.right[12],
                  { top: 12 },
                  gap.all[4],
                ]}
              >
                <PlayerUI.Viewers />
              </View>
            )}

            <View
              style={[
                {
                  padding: 10,
                  backgroundColor: "rgba(0, 0, 0, 0.5)",
                },
                r[2],
                layout.position.absolute,
                position.right[1],
                { top: 8 },
                gap.all[4],
              ]}
            >
              {ingest === null ? (
                <PlayerUI.ContextMenu />
              ) : (
                <Pressable onPress={doSetIngestCamera}>
                  <SwitchCamera size={32} color={colors.gray[200]} />
                </Pressable>
              )}
            </View>

            {shouldShowFloatingMetrics && isLive && (
              <View
                style={[
                  layout.position.absolute,
                  { top: safeAreaInsets.top + 112 },
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
        </Animated.View>
      </TouchableWithoutFeedback>

      {!isSelfAndNotLive && (
        <MobileChatPanel isPlayerRatioGreater={isPlayerRatioGreater} />
      )}
    </>
  );
}
