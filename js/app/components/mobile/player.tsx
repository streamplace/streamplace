import { useNavigation } from "@react-navigation/native";
import {
  Button,
  layout,
  LivestreamProvider,
  Player as PlayerInnerInner,
  PlayerProps,
  PlayerProvider,
  PlayerUI,
  RotationProvider,
  Text,
  useLivestream,
  useLivestreamInfo,
  useLivestreamStore,
  usePlayerDimensions,
  usePlayerStore,
  useSegment,
  useSegmentDimensions,
  VideoProvider,
  View,
  VodSection,
} from "@streamplace/components";
import { gap, h, pt, w } from "@streamplace/components/src/lib/theme/atoms";
import {
  motion,
  borderRadius as radiusTokens,
  spacing as spacingTokens,
} from "@streamplace/components/src/lib/theme/tokens";
import { useLiveUser } from "hooks/useLiveUser";
import { useSidebarControl } from "hooks/useSidebarControl";
import { ArrowLeft } from "lucide-react-native";
import { ComponentRef, useEffect, useRef, useState } from "react";
import {
  Platform,
  ScrollView,
  StatusBar,
  useWindowDimensions,
} from "react-native";
import Reanimated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useUserProfile } from "store/hooks";
import { convertNavigationParams } from "../../src/navigation-helper";
import { BottomMetadata } from "./bottom-metadata";
import { DesktopChatPanel, MobileChatPanel } from "./chat";
import { DesktopUi } from "./desktop-ui";
import { OfflineCounter } from "./offline-counter";
import { MobileUi } from "./ui";
import { useResponsiveLayout } from "./useResponsiveLayout";

import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useStore } from "store";
import { UserOffline } from "./user-offline";

const SEGMENT_TIMEOUT = 500; // half a sec

export function Player(
  props: Partial<PlayerProps> & {
    setFullscreen?: (fullscreen: boolean) => void;
    onTeleport?: (targetHandle: string, targetDID: string) => void;
  },
) {
  const inner = (
    <RotationProvider enabled={Platform.OS !== "web"}>
      <StatusBar hidden={true} />
      <PlayerProvider defaultId={props.playerId || undefined}>
        <PlayerWithProvider {...props} />
      </PlayerProvider>
    </RotationProvider>
  );
  const mode = props.mode ?? "live";
  if (mode === "vod") {
    return (
      <LivestreamProvider src="">
        <VideoProvider aturi={props.src ?? ""}>{inner}</VideoProvider>
      </LivestreamProvider>
    );
  }
  return <LivestreamProvider src={props.src ?? ""}>{inner}</LivestreamProvider>;
}

function PlayerWithProvider(
  props: Partial<PlayerProps> & {
    setFullscreen?: (fullscreen: boolean) => void;
    onTeleport?: (targetHandle: string, targetDID: string) => void;
  },
) {
  // Chat visibility is a persisted preference (survives reloads), except VOD
  // playback which never shows the live chat panel.
  const setShowChat = useStore((state) => state.setChatVisible);
  let showChat = useStore((state) => state.chatVisible);
  if (props.mode === "vod") {
    showChat = false;
  }
  const { shouldShowChatSidePanel, chatPanelWidth } = useResponsiveLayout();
  const chatVisible = shouldShowChatSidePanel && showChat;
  const { width: screenWidth, height: screenHeight } = useWindowDimensions();
  let { top: safeTop } = useSafeAreaInsets();
  const segDims = useSegmentDimensions();
  const isPortrait = screenHeight > screenWidth;
  // if the screen is portrait and video is landscaps
  const isPortraitLandscapeCase =
    isPortrait &&
    segDims.width > segDims.height &&
    !shouldShowChatSidePanel &&
    !props.ingest &&
    props.mode !== "vod";
  const videoBoxHeight = isPortraitLandscapeCase
    ? Math.round((screenWidth * segDims.height) / segDims.width)
    : undefined;

  const websocketConnected = useLivestreamStore((x) => x.websocketConnected);
  const hasReceivedSegment = useLivestreamStore((x) => x.hasReceivedSegment);
  const [showUnavailable, setShowUnavailable] = useState(false);
  const segs = useSegment();

  // periodically check if segment has become stale
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const interval = setInterval(() => {
      setNow(Date.now());
    }, 15000); // check every 15 seconds
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    // don't show unavailable when in ingest mode (you're the one streaming)
    if (props.ingest) {
      setShowUnavailable(false);
      return;
    }

    if (!websocketConnected) {
      setShowUnavailable(false);
      return;
    }

    const then = new Date(segs?.startTime || 0).getTime();
    const segmentIsStale = segs?.startTime ? then < now - 300_000 : true;

    if (!segmentIsStale) {
      setShowUnavailable(false);
      return;
    }

    const timer = setTimeout(() => {
      setShowUnavailable(true);
    }, SEGMENT_TIMEOUT);
    return () => clearTimeout(timer);
  }, [websocketConnected, hasReceivedSegment, segs, now, props.ingest]);

  const [isStreamingElsewhere, setIsStreamingElsewhere] = useState<
    boolean | null
  >(null);
  // are we currently streaming on another device?
  const userIsLive = useLiveUser();
  const userProfile = useUserProfile();

  useEffect(() => {
    if (props.ingest && userIsLive && isStreamingElsewhere === null) {
      setIsStreamingElsewhere(true);
    } else if (props.ingest && userIsLive === false) {
      setIsStreamingElsewhere(false);
    }
  }, [userIsLive]);

  const navigation = useNavigation();

  useEffect(() => {
    return () => {
      StatusBar.setHidden(false, "slide");
    };
  }, []);

  const livestream = useLivestream();
  const localLivestreamURI = useLivestreamStore((x) => x.localLivestreamURI);

  let chatSection: React.ReactNode = null;
  if (props.mode === "vod") {
    chatSection = null;
  } else if (isPortraitLandscapeCase) {
    // Mobile portrait watching a landscape stream: YouTube grammar —
    // video on top, title/streamer row beneath, chat below that.
    chatSection = (
      <>
        <MobileUi hideMobileChat={true} showChat />
        {!props.ingest && !showUnavailable && (
          <BottomMetadata compact setShowChat={setShowChat} showChat />
        )}
        <MobileChatPanel isPlayerRatioGreater={true} fixed={true} />
      </>
    );
  } else if (shouldShowChatSidePanel) {
    chatSection = (
      <DesktopChatPanel
        chatVisible={chatVisible}
        chatPanelWidth={chatPanelWidth}
        setShowChat={setShowChat}
      />
    );
  } else if (!showUnavailable) {
    chatSection = <View />;
  }

  if (isStreamingElsewhere) {
    return (
      <View style={[layout.flex.center, h.percent[100], gap.all[4]]}>
        <Text weight="semibold" size="3xl" style={[pt[2]]}>
          Oeps!
        </Text>
        <View>
          <Text center>You're already streaming from another device.</Text>
          <Text>Please end your other stream before starting one here.</Text>
        </View>
        <View
          style={[
            layout.flex.row,
            w.percent[100],
            gap.column[2],
            layout.flex.center,
          ]}
        >
          <Button
            variant="secondary"
            style={[w.percent[40]]}
            onPress={() => {
              if (navigation.canGoBack()) {
                navigation.goBack();
              } else {
                const params = convertNavigationParams({
                  screen: "HomeMain",
                });
                navigation.navigate(params.screen as any, params.params);
              }
            }}
          >
            <View
              centered
              style={[layout.flex.center, layout.flex.row, gap.all[1]]}
            >
              <ArrowLeft />
              <Text>Back</Text>
            </View>
          </Button>
        </View>
      </View>
    );
  }

  if (props.ingest && livestream && livestream.uri !== localLivestreamURI) {
    return <LivestreamWarning />;
  }

  const defaultHandleTeleport = (targetHandle: string, targetDID: string) => {
    navigation.navigate("Stream", {
      user: targetHandle,
    });
  };

  const handleTeleport = props.onTeleport || defaultHandleTeleport;

  return (
    <RotationProvider enabled={Platform.OS !== "web"}>
      <LivestreamProvider src={props.src ?? ""} onTeleport={handleTeleport}>
        <StatusBar hidden={true} />
        <PlayerProvider defaultId={props.playerId || undefined}>
          <View
            style={[
              {
                flexDirection: chatVisible ? "row" : "column",
                flex: 1,
                width: "100%",
                height: "100%",
                paddingTop:
                  isPortraitLandscapeCase && Platform.OS != "web"
                    ? 54
                    : undefined,
              },
            ]}
          >
            <View
              style={
                isPortraitLandscapeCase
                  ? {
                      height: (videoBoxHeight ?? 0) + safeTop,
                      paddingTop: safeTop,
                    }
                  : { flex: 1 }
              }
            >
              <PlayerInner
                {...props}
                showChat={showChat}
                setShowChat={setShowChat}
                showUnavailable={showUnavailable}
              />
            </View>
            {chatSection}
          </View>
        </PlayerProvider>
      </LivestreamProvider>
    </RotationProvider>
  );
}

export function PlayerInner(
  props: Partial<PlayerProps> & {
    showChat: boolean;
    setShowChat: (show: boolean) => void;
    showUnavailable: boolean;
  },
) {
  let sb = useSidebarControl();
  let fullscreen = usePlayerStore((x) => x.fullscreen);
  const dropdownPortalRef = useRef<ComponentRef<typeof View> | null>(null);
  const {
    shouldShowChatSidePanel,
    chatPanelWidth,
    screenWidth,
    contentWidth,
    availableHeight,
  } = useResponsiveLayout({
    // The actual horizontal space the sidebar reserves from content — 0 in
    // overlay mode (detail views), so the player fills the width up to the chat.
    // Plain reactive number (not the shared value) so the width is correct on
    // the first render, including a direct page load.
    sidebarWidth: sb.contentMargin,
    sidebarHidden: !sb.isActive,
    showChatSidePanelOnLandscape: props.showChat,
  });

  const safeAreaInsets = useSafeAreaInsets();
  const setSidebarHidden = useStore((state) => state.setSidebarHidden);
  const setSidebarUnhidden = useStore((state) => state.setSidebarUnhidden);

  // auto-collapse chat once when going offline
  const hasCollapsedChat = useRef(false);
  useEffect(() => {
    if (
      props.showUnavailable &&
      shouldShowChatSidePanel &&
      !hasCollapsedChat.current
    ) {
      props.setShowChat(false);
      hasCollapsedChat.current = true;
    }
    if (!props.showUnavailable) {
      hasCollapsedChat.current = false;
    }
  }, [props.showUnavailable, shouldShowChatSidePanel]);

  // animated height for offline state
  const heightMultiplier = useSharedValue(1);

  useEffect(() => {
    if (props.showUnavailable) {
      heightMultiplier.value = withTiming(0.65, { duration: motion.slow });
    } else {
      heightMultiplier.value = withTiming(1, { duration: motion.slow });
    }
  }, [props.showUnavailable]);

  // content info
  const { width: pwidth, height: pheight } = usePlayerDimensions();

  let width = pwidth > 0 && props.mode !== "vod" ? pwidth : 16;
  let height = pheight > 0 && props.mode !== "vod" ? pheight : 9;

  // Calculate aspect ratio and determine if we're in desktop mode
  const aspectRatio = width > 0 && height > 0 ? width / height : 16 / 9;

  // The VOD box is sized to the real video aspect ratio (so portrait and other
  // shapes aren't forced into 16:9), falling back to 16:9 until the track
  // metadata loads.
  const segDims = useSegmentDimensions();
  const vodAspectRatio =
    segDims.width > 0 && segDims.height > 0
      ? segDims.width / segDims.height
      : 16 / 9;

  // on mobile we want to hide the sidebar when going fullscreen
  useEffect(() => {
    if (Platform.OS !== "web" && width > height) {
      console.log("hiding sb");
      setSidebarHidden();
    } else {
      setSidebarUnhidden();
    }
    return () => {
      setSidebarUnhidden();
    };
  }, [width, height]);
  // should cover full width on mobile?
  const isDesktopMode = shouldShowChatSidePanel || screenWidth > 1200;

  // Calculate optimal height for desktop mode (90% of available height)
  const maxDesktopHeight = availableHeight * 0.8;
  const chatVisible = shouldShowChatSidePanel && props.showChat;

  const showFullDesktopMode = aspectRatio > 1 && screenWidth > 1200;
  const isLandscape = aspectRatio > 1;

  // Desktop theater framing: the player floats in a padded well with
  // rounded corners (YouTube grammar) instead of running edge-to-edge.
  const playerPad = showFullDesktopMode && !fullscreen ? spacingTokens[6] : 0;

  // Web VOD pages have a sticky translucent header (56px) rendered by the shell;
  // reserve space so the video starts below it and scrolls under it.
  const vodHeaderPad =
    Platform.OS === "web" &&
    props.mode === "vod" &&
    sb.isActive &&
    !fullscreen
      ? 56
      : 0;

  const calculatedWidth =
    (chatVisible ? contentWidth - chatPanelWidth : contentWidth) -
    playerPad * 2;

  const calculatedHeight = isDesktopMode
    ? Math.min(calculatedWidth / aspectRatio, maxDesktopHeight)
    : height;

  // When fullscreen or the device is rotated to landscape, a width-100% +
  // aspectRatio VOD box is taller than the screen and clips off the bottom.
  // In those cases fill the area and let objectFit:contain letterbox instead.
  const { width: winWidth, height: winHeight } = useWindowDimensions();
  const vodFillScreen = fullscreen || winWidth > winHeight;

  const isPlayerRatioGreater = aspectRatio >= 16 / 9;

  // animated style for offline height transition
  const animatedHeightStyle = useAnimatedStyle(() => {
    return {
      height: showFullDesktopMode
        ? calculatedHeight * heightMultiplier.value
        : undefined,
    };
  });

  const videoContent = props.showUnavailable ? (
    <UserOffline />
  ) : (
    <PlayerInnerInner {...props}>
      {showFullDesktopMode || fullscreen ? (
        <DesktopUi dropdownPortalContainer={dropdownPortalRef.current} />
      ) : (
        (isLandscape || props.mode === "vod") && (
          <MobileUi
            hideMobileChat={props.mode === "vod"}
            setShowChat={props.setShowChat}
            showChat={props.showChat}
          />
        )
      )}
      <PlayerUI.ViewerLoadingOverlay />
      {props.mode !== "vod" && !props.showUnavailable && (
        <OfflineCounter isMobile={true} />
      )}
      <View
        ref={dropdownPortalRef}
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          pointerEvents: "none",
        }}
      />
    </PlayerInnerInner>
  );

  // Mobile inline VOD: keep the video + metadata header fixed and scroll only
  // the description below the line, so the video is always visible and a long
  // description can't flex-shrink the player.
  if (props.mode === "vod" && !showFullDesktopMode && !fullscreen) {
    return (
      <View style={{ flex: 1, paddingTop: safeAreaInsets.top + vodHeaderPad }}>
        <Reanimated.View
          style={{
            width: "100%",
            aspectRatio: vodAspectRatio,
            // Cap so a landscape video fits (letterboxed via objectFit:contain)
            // instead of clipping; flexShrink:0 keeps it from collapsing.
            maxHeight: isLandscape ? winHeight : winHeight * 0.7,
            // center the video horizontally when in landscape since it won't fill the full width
            marginHorizontal: isLandscape
              ? (winWidth - Math.min(winWidth, winHeight * vodAspectRatio)) / 2
              : 0,
            flexShrink: 0,
          }}
        >
          {videoContent}
        </Reanimated.View>
        {/* will get pushed below the video if landscape so probably fine? */}
        <VodSection scrollDescription />
      </View>
    );
  }

  return (
    <ScrollView
      style={{
        height: showFullDesktopMode ? "100%" : undefined,
        flex: 1,
        maxWidth: calculatedWidth + playerPad * 2,
      }}
      contentContainerStyle={
        showFullDesktopMode
          ? {
              flexGrow: 1, // This makes content expand to fill available space
              minHeight: "100%", // Ensures minimum height
              ...(fullscreen
                ? {}
                : {
                    paddingHorizontal: playerPad,
                    paddingTop: spacingTokens[4] + vodHeaderPad,
                  }),
            }
          : props.mode === "vod"
            ? {
                flexGrow: 1,
                paddingTop: safeAreaInsets.top + vodHeaderPad,
              }
            : {
                flex: 1,
              }
      }
      scrollEnabled={showFullDesktopMode || props.mode === "vod"}
      bounces={false}
      showsVerticalScrollIndicator={false}
    >
      <Reanimated.View
        style={[
          showFullDesktopMode
            ? {
                width: calculatedWidth,
                ...(fullscreen
                  ? {}
                  : {
                      borderRadius: radiusTokens.lg,
                      overflow: "hidden" as const,
                    }),
              }
            : props.mode === "vod"
              ? vodFillScreen
                ? // Fullscreen/landscape: fill the area; objectFit:contain
                  // letterboxes so the video fits without clipping.
                  { flex: 1 }
                : {
                    // Portrait inline: bound the video to its real aspect ratio
                    // so it occupies a fixed height with the metadata below —
                    // never the whole window. (A pixel height derived from
                    // contentWidth collapsed to full-window on Android when
                    // contentWidth measured 0.)
                    width: "100%" as any,
                    aspectRatio: vodAspectRatio,
                  }
              : {
                  flex: 1,
                  maxHeight: "auto",
                },
          {
            paddingTop:
              isPlayerRatioGreater && !isLandscape && !props.showUnavailable
                ? safeAreaInsets.top
                : 0,
          },
          animatedHeightStyle,
        ]}
      >
        {videoContent}
      </Reanimated.View>
      {showFullDesktopMode && props.mode !== "vod" && (
        <BottomMetadata
          setShowChat={props.setShowChat}
          showChat={props.showChat}
        />
      )}
      {props.mode === "vod" && <VodSection />}
    </ScrollView>
  );
}

export function LivestreamWarning() {
  const livestream = useLivestream();
  const localLivestreamURI = useLivestreamStore((x) => x.localLivestreamURI);
  const { toggleStopStream } = useLivestreamInfo();
  const navigation = useNavigation();
  const setLocalLivestreamURI = useLivestreamStore(
    (x) => x.setLocalLivestreamURI,
  );

  const [loading, setLoading] = useState(false);

  if (livestream && livestream.uri !== localLivestreamURI) {
    return (
      <View style={[layout.flex.center, h.percent[100], gap.all[4]]}>
        <Text size="xl">You have an active livestream!</Text>
        <Text>"{livestream.record.title}"</Text>
        <Button
          style={[w.percent[60]]}
          onPress={() => {
            setLoading(true);
            setLocalLivestreamURI(livestream.uri);
          }}
          disabled={loading}
        >
          <View
            centered
            style={[layout.flex.center, layout.flex.row, gap.all[1]]}
          >
            <Text center>Resume that stream from here</Text>
          </View>
        </Button>
        <Button
          style={[w.percent[60]]}
          onPress={async () => {
            setLoading(true);
            try {
              await toggleStopStream();
            } catch (error) {
              console.error(error);
            } finally {
              // we want to keep loading until the firehose tells us the stream is stopped
            }
          }}
          variant="danger"
          disabled={loading}
        >
          <View
            centered
            style={[layout.flex.center, layout.flex.row, gap.all[1]]}
          >
            <Text center>End that stream and start a new one</Text>
          </View>
        </Button>
        <Button
          variant="secondary"
          style={[w.percent[60]]}
          onPress={() =>
            navigation.navigate("MainTabs" as any, { screen: "HomeTab" })
          }
        >
          <View
            centered
            style={[layout.flex.center, layout.flex.row, gap.all[1]]}
          >
            <Text>Back</Text>
          </View>
        </Button>
      </View>
    );
  }
}
