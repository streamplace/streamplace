import {
  PlayerStatus,
  PlayerUI,
  PortalHost,
  Toast,
  useLivestreamInfo,
  useOffline,
  usePlayerDimensions,
  usePlayerStore,
  useSegment,
  View,
  zero,
} from "@streamplace/components";
import { AnimatedGradient } from "components/ui/gradient";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { Platform } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import {
  BottomControlBar,
  MuteOverlay,
  TopControlBar,
} from "./desktop-ui/index";
import { PlayPauseIndicator } from "./desktop-ui/play-pause-indicator";
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
  isChatOpen,
  setIsChatOpen,
}: {
  dropdownPortalContainer?: any;
  isChatOpen?: boolean;
  setIsChatOpen?: (open: boolean) => void;
}) {
  const {
    ingest,
    title,
    setTitle,
    showCountdown,
    setShowCountdown,
    recordSubmitted,
    setRecordSubmitted,
    toggleGoLive,
  } = useLivestreamInfo();
  const { width, height } = usePlayerDimensions();
  const { shouldShowFloatingMetrics } = useResponsiveLayout();
  const playerId = usePlayerStore((state) => state.id);

  const originalSafeAreaInsets = useSafeAreaInsets();

  const offline = useOffline();
  const showMetrics = usePlayerStore((state) => state.showDebugInfo);
  const pipAction = usePlayerStore((state) => state.pipAction);
  const videoRef = usePlayerStore((state) => state.videoRef);
  const embedded = usePlayerStore((state) => state.embedded);

  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const setFullscreen = usePlayerStore((state) => state.setFullscreen);
  const selectedRendition = usePlayerStore((state) => state.selectedRendition);
  const status = usePlayerStore((state) => state.status);

  const safeAreaInsets = embedded
    ? { ...originalSafeAreaInsets, top: 0 }
    : originalSafeAreaInsets;

  const segment = useSegment();

  const [isControlsVisible, setIsControlsVisible] = useState(true);
  const [pipSupported, setPipSupported] = useState(false);
  const [pipActive, setPipActive] = useState(false);
  const fadeOpacity = useSharedValue(1);
  const fadeTimeout = useRef<NodeJS.Timeout | null>(null);
  const FADE_OUT_DELAY = 2500;

  const isSelfAndNotLive = ingest === "new";
  const isActivelyLive = ingest !== null && ingest !== "new";

  const resetFadeTimer = useCallback(() => {
    fadeOpacity.value = withTiming(1, { duration: 200 });
    if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    setIsControlsVisible(true);

    if (selectedRendition === "audio") return;
    if (ingest !== null) return;
    if (status === PlayerStatus.PAUSE) return;

    fadeTimeout.current = setTimeout(() => {
      fadeOpacity.value = withTiming(0, { duration: 400 });
      setIsControlsVisible(false);
    }, FADE_OUT_DELAY);
  }, [fadeOpacity, selectedRendition, ingest, status]);

  const onPlayerHover = useCallback(() => {
    resetFadeTimer();
  }, [resetFadeTimer]);

  const toggleChat = useCallback(() => {
    if (setIsChatOpen) setIsChatOpen(!isChatOpen);
  }, []);

  const toggleFullscreen = useCallback(() => {
    setFullscreen(!fullscreen);
  }, [fullscreen, setFullscreen]);

  useEffect(() => {
    resetFadeTimer();

    return () => {
      if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    };
  }, [resetFadeTimer]);

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

  // Keyboard shortcuts (F for fullscreen)
  useEffect(() => {
    if (Platform.OS !== "web") return;

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "f" || e.key === "F") {
        // are we in an input/textarea or contenteditable element?
        const activeEl = document.activeElement;
        const isInput =
          activeEl &&
          (activeEl.tagName === "INPUT" ||
            activeEl.tagName === "TEXTAREA" ||
            (activeEl as HTMLElement).isContentEditable);
        if (isInput) return;
        e.preventDefault();
        toggleFullscreen();
      }
    }

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [toggleFullscreen]);

  const handlePip = useCallback(() => {
    if (pipAction) pipAction();
  }, [pipAction]);

  const hover = Gesture.Hover().onChange((_) => runOnJS(onPlayerHover)());

  const togglePlayPause = usePlayerStore((x) => x.togglePlayPause);

  const handleSingleClick = useCallback(() => {
    togglePlayPause();
  }, [togglePlayPause]);

  const handleDoubleClick = useCallback(() => {
    toggleFullscreen();
  }, [toggleFullscreen]);

  const singleTap = Gesture.Tap().onEnd(() => runOnJS(handleSingleClick)());

  const doubleTap = Gesture.Tap()
    .numberOfTaps(2)
    .onEnd(() => runOnJS(handleDoubleClick)());

  const tap = Gesture.Exclusive(doubleTap, singleTap);
  const hoverAndTap = Gesture.Race(hover, tap);

  const portalContainerID = "desktop-ui-dropdown-portal-" + playerId;

  return (
    <>
      <GestureDetector gesture={hoverAndTap}>
        <View
          style={[layout.position.absolute, h.percent[100], w.percent[100]]}
          collapsable={false}
        >
          <PlayerUI.ViewerLoadingOverlay />
          <PlayPauseIndicator />
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
              isChatOpen={isChatOpen || false}
              onToggleChat={toggleChat}
              embedded={embedded}
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

          {isSelfAndNotLive && (
            <PlayerUI.InputPanel
              title={title}
              setTitle={setTitle}
              toggleGoLive={toggleGoLive}
              isLive={isActivelyLive}
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
        </View>
      </GestureDetector>
      <MuteOverlay />
      <Animated.View
        style={[
          layout.position.absolute,
          position.bottom[0],
          w.percent[100],
          { zIndex: 999 },
          animatedFadeStyle,
          // no clickthrough
          { pointerEvents: isControlsVisible ? "auto" : "none" },
        ]}
      >
        <AnimatedGradient
          fromColor="#00000080"
          toColor="#000000"
          opacityColor1={0}
        >
          <BottomControlBar
            ingest={ingest}
            pipSupported={pipSupported}
            pipActive={pipActive}
            onHandlePip={handlePip}
            dropdownPortalContainer={fullscreen && portalContainerID}
            showChat={isChatOpen || false}
            setShowChat={setIsChatOpen || undefined}
          />
        </AnimatedGradient>
      </Animated.View>
      {fullscreen && <PortalHost name={portalContainerID} />}
    </>
  );
}
