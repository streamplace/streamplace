import {
  PlayerStatus,
  PlayerUI,
  PortalHost,
  Toast,
  useLivestreamInfo,
  useOffline,
  usePlayerDimensions,
  usePlayerStore,
  View,
  zero,
} from "@streamplace/components";
import {
  borderAlphas,
  borderRadius,
  colors,
  motion,
  scrims,
  spacing,
} from "@streamplace/components/src/lib/theme/tokens";
import Gradient from "components/ui/gradient";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { Platform, type PointerEvent } from "react-native";
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

const { h, layout, position, w, px, py } = zero;

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
  const mode = usePlayerStore((state) => state.mode);
  const hasEnded = usePlayerStore(
    (state) =>
      state.mode === "vod" &&
      state.status !== PlayerStatus.PLAYING &&
      state.duration > 0 &&
      state.playTime >= state.duration,
  );

  const safeAreaInsets = embedded
    ? { ...originalSafeAreaInsets, top: 0 }
    : originalSafeAreaInsets;

  const [isControlsVisible, setIsControlsVisible] = useState(true);
  const controlsVisibleRef = useRef(true);
  const pointerType = useRef("mouse");
  const keyboardInput = useRef(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [scrubbing, setScrubbing] = useState(false);
  const [controlsFocused, setControlsFocused] = useState(false);
  const [pipSupported, setPipSupported] = useState(false);
  const [pipActive, setPipActive] = useState(false);
  const fadeOpacity = useSharedValue(1);
  const fadeTimeout = useRef<NodeJS.Timeout | null>(null);
  const FADE_OUT_DELAY = 6000;

  const isSelfAndNotLive = ingest === "new";
  const isActivelyLive = ingest !== null && ingest !== "new";

  const hideControls = useCallback(() => {
    if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    fadeOpacity.value = withTiming(0, { duration: motion.slow });
    setIsControlsVisible(false);
    controlsVisibleRef.current = false;
  }, [fadeOpacity]);

  const resetFadeTimer = useCallback(() => {
    fadeOpacity.value = withTiming(1, { duration: motion.base });
    if (fadeTimeout.current) clearTimeout(fadeTimeout.current);
    setIsControlsVisible(true);
    controlsVisibleRef.current = true;

    if (selectedRendition === "audio") return;
    if (ingest !== null) return;
    if (
      status === PlayerStatus.PAUSE ||
      hasEnded ||
      settingsOpen ||
      scrubbing ||
      (controlsFocused && keyboardInput.current)
    )
      return;

    fadeTimeout.current = setTimeout(hideControls, FADE_OUT_DELAY);
  }, [
    fadeOpacity,
    selectedRendition,
    ingest,
    status,
    hasEnded,
    settingsOpen,
    scrubbing,
    controlsFocused,
    hideControls,
  ]);

  const onPlayerHover = useCallback(() => {
    resetFadeTimer();
  }, [resetFadeTimer]);

  const toggleChat = useCallback(() => {
    if (setIsChatOpen) setIsChatOpen(!isChatOpen);
  }, [isChatOpen, setIsChatOpen]);

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
    opacity: fadeOpacity.value,
  }));

  const onPlayerPointerDown = (event: PointerEvent) => {
    pointerType.current = event.nativeEvent.pointerType || "mouse";
    keyboardInput.current = false;
  };

  const onControlsPointerDown = (event: PointerEvent) => {
    onPlayerPointerDown(event);
    resetFadeTimer();
  };

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
      keyboardInput.current = true;
      if (controlsFocused) resetFadeTimer();
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
        resetFadeTimer();
      }
    }

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [toggleFullscreen, controlsFocused, resetFadeTimer]);

  const handlePip = useCallback(() => {
    if (pipAction) pipAction();
  }, [pipAction]);

  const hover = Gesture.Hover().onChange((_) => runOnJS(onPlayerHover)());

  const togglePlayPause = usePlayerStore((x) => x.togglePlayPause);

  const handleSingleClick = useCallback(() => {
    if (pointerType.current !== "mouse") {
      if (controlsVisibleRef.current && ingest === null) hideControls();
      else resetFadeTimer();
      return;
    }
    if (!controlsVisibleRef.current) {
      resetFadeTimer();
      return;
    }
    // Click-to-pause is for on-demand video. A live stream has nothing to
    // pause into, and on phones the tap that reaches for a control button
    // was landing here and pausing the stream instead.
    if (mode !== "vod") {
      resetFadeTimer();
      return;
    }
    togglePlayPause();
    resetFadeTimer();
  }, [togglePlayPause, resetFadeTimer, hideControls, ingest, mode]);

  const handleDoubleClick = useCallback(() => {
    toggleFullscreen();
    resetFadeTimer();
  }, [toggleFullscreen, resetFadeTimer]);

  const singleTap = Gesture.Tap().onEnd((_event, success) => {
    if (success) runOnJS(handleSingleClick)();
  });

  const doubleTap = Gesture.Tap()
    .numberOfTaps(2)
    .onEnd((_event, success) => {
      if (success) runOnJS(handleDoubleClick)();
    });

  const tap = Gesture.Exclusive(doubleTap, singleTap);
  const hoverAndTap = Gesture.Race(hover, tap);

  const portalContainerID = "desktop-ui-dropdown-portal-" + playerId;

  return (
    <>
      <View
        pointerEvents="box-none"
        style={[
          layout.position.absolute,
          h.percent[100],
          w.percent[100],
          { overflow: "hidden" },
        ]}
      >
        <GestureDetector gesture={hoverAndTap}>
          <View
            testID="desktop-player-surface"
            onPointerDown={onPlayerPointerDown}
            style={[layout.position.absolute, h.percent[100], w.percent[100]]}
            collapsable={false}
          />
        </GestureDetector>
        <PlayerUI.ViewerLoadingOverlay />
        <PlayPauseIndicator />
        <Animated.View
          testID="desktop-player-header"
          pointerEvents={isControlsVisible ? "box-none" : "none"}
          aria-hidden={!isControlsVisible}
          accessibilityElementsHidden={!isControlsVisible}
          importantForAccessibility={
            isControlsVisible ? "auto" : "no-hide-descendants"
          }
          onFocus={() => setControlsFocused(true)}
          onBlur={() => setControlsFocused(false)}
          onPointerDown={onControlsPointerDown}
          onPointerMove={onPlayerHover}
          style={[
            layout.position.absolute,
            position.top[0],
            w.percent[100],
            animatedFadeStyle,
          ]}
        >
          <Gradient
            pointerEvents="box-none"
            fromColor={scrims.dark}
            toColor={colors.black}
            opacityColor2={0}
          >
            <View
              pointerEvents="box-none"
              style={{
                paddingTop: safeAreaInsets.top + spacing[1],
                paddingHorizontal: spacing[2],
                paddingBottom: spacing[2],
              }}
            >
              <TopControlBar
                offline={offline}
                isActivelyLive={isActivelyLive}
                ingest={ingest}
                isChatOpen={isChatOpen || false}
                onToggleChat={toggleChat}
                embedded={embedded}
              />
            </View>
          </Gradient>
        </Animated.View>

        {isActivelyLive && isControlsVisible && (
          <Animated.View
            style={[
              layout.position.absolute,
              {
                top: safeAreaInsets.top + spacing[16],
                left: spacing[4],
              },
              animatedFadeStyle,
            ]}
          >
            {/* Connection-health HUD. `showMetrics` (debug flag) gates the
                  raw Δ/mean/range readout — never shown to broadcasters by
                  default; the pill itself is always visible while live. */}
            <PlayerUI.MetricsPanel showMetrics={showMetrics} />
          </Animated.View>
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
                backgroundColor: scrims.dark,
                borderRadius: borderRadius.md,
                borderWidth: 1,
                borderColor: borderAlphas.dark.strong,
              },
            ]}
          >
            <PlayerUI.MetricsPanel showMetrics={showMetrics} />
          </View>
        )}
      </View>
      <MuteOverlay />
      <Animated.View
        testID="desktop-player-controls"
        pointerEvents={isControlsVisible ? "box-none" : "none"}
        aria-hidden={!isControlsVisible}
        accessibilityElementsHidden={!isControlsVisible}
        importantForAccessibility={
          isControlsVisible ? "auto" : "no-hide-descendants"
        }
        onFocus={() => setControlsFocused(true)}
        onBlur={() => setControlsFocused(false)}
        onPointerDown={onControlsPointerDown}
        onPointerMove={onPlayerHover}
        style={[
          layout.position.absolute,
          position.bottom[0],
          w.percent[100],
          { zIndex: 999 },
          animatedFadeStyle,
        ]}
      >
        <Gradient
          pointerEvents="box-none"
          fromColor={colors.black}
          toColor={scrims.dark}
          opacityColor1={0}
          style={{ paddingBottom: spacing[2] }}
        >
          <BottomControlBar
            ingest={ingest}
            pipSupported={pipSupported}
            pipActive={pipActive}
            onHandlePip={handlePip}
            dropdownPortalContainer={fullscreen ? portalContainerID : undefined}
            showChat={isChatOpen || false}
            setShowChat={setIsChatOpen || undefined}
            onMenuOpenChange={setSettingsOpen}
            onScrubbingChange={setScrubbing}
          />
        </Gradient>
      </Animated.View>
      {fullscreen && <PortalHost name={portalContainerID} />}
    </>
  );
}
