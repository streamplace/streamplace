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
  useLivestreamStore,
  usePlayerDimensions,
  usePlayerStore,
  useSegment,
  View,
} from "@streamplace/components";
import { gap, h, pt, w } from "@streamplace/components/src/lib/theme/atoms";
import { useLiveUser } from "hooks/useLiveUser";
import { useSidebarControl } from "hooks/useSidebarControl";
import { ArrowLeft, ArrowRight } from "lucide-react-native";
import { ComponentRef, useEffect, useRef, useState } from "react";
import { Platform, ScrollView, StatusBar } from "react-native";
import Reanimated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { useUserProfile } from "store/hooks";
import { BottomMetadata } from "./bottom-metadata";
import { DesktopChatPanel } from "./chat";
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
  },
) {
  const [showChat, setShowChat] = useState(true);
  const { shouldShowChatSidePanel, chatPanelWidth } = useResponsiveLayout();
  const chatVisible = shouldShowChatSidePanel && showChat;

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
  }, [websocketConnected, hasReceivedSegment, segs, now]);

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
            onPress={() =>
              navigation.canGoBack()
                ? navigation.goBack()
                : navigation.navigate("Home", { screen: "StreamList" })
            }
          >
            <View
              centered
              style={[layout.flex.center, layout.flex.row, gap.all[1]]}
            >
              <ArrowLeft />
              <Text>Back</Text>
            </View>
          </Button>
          {userProfile?.did && (
            <Button
              style={[w.percent[40]]}
              onPress={() =>
                navigation.navigate("Home", {
                  screen: "Stream",
                  params: { user: userProfile?.did },
                })
              }
            >
              <View
                centered
                style={[layout.flex.center, layout.flex.row, gap.all[1]]}
              >
                <Text>Your stream</Text>
                <ArrowRight />
              </View>
            </Button>
          )}
        </View>
      </View>
    );
  }

  return (
    <RotationProvider enabled={Platform.OS !== "web"}>
      <LivestreamProvider src={props.src ?? ""}>
        <StatusBar hidden={true} />
        <PlayerProvider defaultId={props.playerId || undefined}>
          <View
            style={{
              flexDirection: chatVisible ? "row" : "column",
              flex: 1,
              width: "100%",
              height: "100%",
            }}
          >
            <PlayerInner
              {...props}
              showChat={showChat}
              setShowChat={setShowChat}
              showUnavailable={showUnavailable}
            />
            {shouldShowChatSidePanel ? (
              <DesktopChatPanel
                chatVisible={chatVisible}
                chatPanelWidth={chatPanelWidth}
              />
            ) : (
              !showUnavailable && <MobileUi />
            )}
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
    sidebarWidth: sb.animatedWidth,
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
      heightMultiplier.value = withTiming(0.65, { duration: 500 });
    } else {
      heightMultiplier.value = withTiming(1, { duration: 500 });
    }
  }, [props.showUnavailable]);

  // content info
  const { width, height } = usePlayerDimensions();

  // Calculate aspect ratio and determine if we're in desktop mode
  const aspectRatio = width > 0 && height > 0 ? width / height : 16 / 9;

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

  const calculatedWidth = chatVisible
    ? contentWidth - chatPanelWidth
    : contentWidth;

  const calculatedHeight = isDesktopMode
    ? Math.min(calculatedWidth / aspectRatio, maxDesktopHeight)
    : height;

  const showFullDesktopMode = aspectRatio > 1 && screenWidth > 1200;
  const isLandscape = aspectRatio > 1;

  const isPlayerRatioGreater = aspectRatio >= 16 / 9;

  // animated style for offline height transition
  const animatedHeightStyle = useAnimatedStyle(() => {
    return {
      height: showFullDesktopMode
        ? calculatedHeight * heightMultiplier.value
        : undefined,
    };
  });

  return (
    <ScrollView
      style={{
        height: showFullDesktopMode ? "100%" : undefined,
        flex: showFullDesktopMode ? 1 : undefined,
        maxWidth: calculatedWidth,
      }}
      contentContainerStyle={
        showFullDesktopMode
          ? {
              flexGrow: 1, // This makes content expand to fill available space
              minHeight: "100%", // Ensures minimum height
            }
          : {
              flex: 1,
            }
      }
      scrollEnabled={showFullDesktopMode}
      bounces={false}
      showsVerticalScrollIndicator={false}
    >
      <Reanimated.View
        style={[
          showFullDesktopMode
            ? {
                width: calculatedWidth,
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
        {props.showUnavailable ? (
          <UserOffline />
        ) : (
          <PlayerInnerInner {...props}>
            {showFullDesktopMode || fullscreen ? (
              <DesktopUi dropdownPortalContainer={dropdownPortalRef.current} />
            ) : (
              isLandscape && (
                <MobileUi
                  setShowChat={props.setShowChat}
                  showChat={props.showChat}
                />
              )
            )}
            <PlayerUI.ViewerLoadingOverlay />
            {!props.showUnavailable && <OfflineCounter isMobile={true} />}
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
        )}
      </Reanimated.View>
      {showFullDesktopMode && (
        <BottomMetadata
          setShowChat={props.setShowChat}
          showChat={props.showChat}
        />
      )}
    </ScrollView>
  );
}
