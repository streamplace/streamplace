import {
  PlayerUI,
  Toast,
  useLivestreamInfo,
  useOffline,
  usePlayerDimensions,
  usePlayerStore,
  useSegment,
  View,
  zero,
} from "@streamplace/components";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { Platform } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import {
  BottomControlBar,
  MuteOverlay,
  TopControlBar,
} from "./desktop-ui/index";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { h, layout, position, w, px, py, r, p } = zero;

function isRefObject(
  ref: any,
): ref is
  | React.RefObject<HTMLVideoElement>
  | React.MutableRefObject<HTMLVideoElement | null> {
  return ref && typeof ref === "object" && "current" in ref;
}

export function DesktopUi({
  dropdownPortalContainer,
}: {
  dropdownPortalContainer?: any;
}) {
  const {
    ingest,
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
  const { safeAreaInsets, shouldShowFloatingMetrics } = useResponsiveLayout();

  const offline = useOffline();
  const showMetrics = usePlayerStore((state) => state.showDebugInfo);
  const pipAction = usePlayerStore((state) => state.pipAction);
  const videoRef = usePlayerStore((state) => state.videoRef);

  const segment = useSegment();

  const [isControlsVisible, setIsControlsVisible] = useState(true);
  const [isChatOpen, setIsChatOpen] = useState(false);
  const [pipSupported, setPipSupported] = useState(false);
  const [pipActive, setPipActive] = useState(false);
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

  // Picture-in-Picture support detection
  useEffect(() => {
    if (Platform.OS === "web") {
      setPipSupported(
        !!document.pictureInPictureEnabled && pipAction !== undefined,
      );
    }
  }, [pipAction]);

  // Picture-in-Picture event listeners
  useEffect(() => {
    if (Platform.OS !== "web") return;

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
  }, [pipAction]);

  // Live timer for offline overlay
  const [timeSinceLastSeen, setTimeSinceLastSeen] = useState("Unknown");

  useEffect(() => {
    if (!offline || !segment?.startTime) {
      setTimeSinceLastSeen("Unknown");
      return;
    }

    const updateTimer = () => {
      const now = new Date();
      const lastSeen = new Date(segment.startTime);
      const diffMs = now.getTime() - lastSeen.getTime();
      const diffMinutes = Math.floor(diffMs / 60000);
      const diffSeconds = Math.floor((diffMs % 60000) / 1000);

      if (diffMinutes > 0) {
        setTimeSinceLastSeen(`${diffMinutes}m ${diffSeconds}s ago`);
      } else {
        setTimeSinceLastSeen(`${diffSeconds}s ago`);
      }
    };

    // Update immediately
    updateTimer();

    // Update every second while offline
    const interval = setInterval(updateTimer, 1000);

    return () => clearInterval(interval);
  }, [offline, segment?.startTime]);

  const hover = Gesture.Hover().onChange((_) => runOnJS(onPlayerHover)());

  return (
    <GestureDetector gesture={hover}>
      <>
        <View
          style={[layout.position.absolute, h.percent[100], w.percent[100]]}
        >
          <MuteOverlay />
          <PlayerUI.AutoplayButton />
          <PlayerUI.ViewerLoadingOverlay />
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
            <TopControlBar
              offline={offline}
              isActivelyLive={isActivelyLive}
              ingest={ingest}
              isChatOpen={isChatOpen}
              onToggleChat={toggleChat}
              safeAreaInsets={safeAreaInsets}
            />
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
            <BottomControlBar
              ingest={ingest}
              pipSupported={pipSupported}
              pipActive={pipActive}
              onHandlePip={handlePip}
              dropdownPortalContainer={dropdownPortalContainer}
            />
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
        </View>
        {showMetrics && (
          <View
            style={[
              layout.position.absolute,
              position.top[20],
              position.left[4],
              px[4],
              py[2],
              {
                backgroundColor: "rgba(0, 0, 0, 0.7)",
                borderRadius: 8,
                borderWidth: 1,
                borderColor: "#374151",
              },
            ]}
          >
            <PlayerUI.MetricsPanel showMetrics={showMetrics} />
          </View>
        )}
      </>
    </GestureDetector>
  );
}
