import { useNavigation } from "@react-navigation/native";
import {
  PlayerUI,
  Text,
  Toast,
  useAvatars,
  useCameraToggle,
  useLivestreamInfo,
  usePlayerDimensions,
  View,
  zero,
} from "@streamplace/components";
import { ChevronLeft, SwitchCamera } from "lucide-react-native";
import { useEffect, useRef } from "react";
import { Image, Pressable, TouchableWithoutFeedback } from "react-native";
import {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { borders, colors, gap, h, layout, position, w, px, py, r } = zero;

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
    <TouchableWithoutFeedback>
      <View style={[layout.position.absolute, h.percent[100], w.percent[100]]}>
        <View
          style={[layout.position.absolute, h.percent[100], w.percent[100]]}
        >
          <View
            style={[
              {
                padding: 8,
                backgroundColor: "rgba(0, 0, 0, 0.6)",
              },
              r[2],
              layout.position.absolute,
              position.left[2],
              { top: safeAreaInsets.top + 12 },
            ]}
          >
            <View style={[layout.flex.row, layout.flex.center, gap.all[3]]}>
              <Pressable
                onPress={() => {
                  navigation.canGoBack()
                    ? navigation.goBack()
                    : navigation.navigate("Home", { screen: "StreamList" });
                }}
              >
                <ChevronLeft color="white" size={24} />
              </Pressable>
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
                    backgroundColor: "green",
                    borderRadius: 20,
                  },
                  borders.width.thin,
                  borders.color.gray[700],
                ]}
              />
              <Text style={{ fontSize: 16, fontWeight: "600", color: "white" }}>
                {profile?.handle}
              </Text>
            </View>
          </View>

          {isLive && (
            <View
              style={[
                layout.position.absolute,
                { top: safeAreaInsets.top + 12 },
                position.left[0],
                position.right[0],
                layout.flex.column,
                layout.flex.center,
              ]}
            >
              <View
                style={[
                  {
                    padding: 12,
                    backgroundColor: "rgba(0, 0, 0, 0.5)",
                  },
                  r[3],
                  layout.flex.row,
                  layout.flex.center,
                  gap.all[4],
                ]}
              >
                <PlayerUI.Viewers />
                <PlayerUI.MetricsPanel showMetrics={isLive} />
              </View>
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
              position.right[2],
              { top: safeAreaInsets.top + 12 },
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
      </View>
    </TouchableWithoutFeedback>
  );
}
