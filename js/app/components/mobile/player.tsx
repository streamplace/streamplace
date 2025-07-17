import { useNavigation } from "@react-navigation/native";
import {
  Button,
  layout,
  LivestreamProvider,
  Player as PlayerInnerInner,
  PlayerProps,
  PlayerProvider,
  Text,
  usePlayerDimensions,
  usePlayerStore,
  View,
} from "@streamplace/components";
import { gap, h, pt, w } from "@streamplace/components/src/lib/theme/atoms";
import { ArrowLeft, ArrowRight } from "@tamagui/lucide-icons";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import { useLiveUser } from "hooks/useLiveUser";
import { useSidebarControl } from "hooks/useSidebarControl";
import { useEffect, useState } from "react";
import { Animated, ScrollView } from "react-native";
import { useAppSelector } from "store/hooks";
import { BottomMetadata } from "./bottom-metadata";
import { DesktopChatPanel } from "./chat";
import { DesktopUi } from "./desktop-ui";
import { OfflineCounter } from "./offline-counter";
import { MobileUi } from "./ui";
import { useResponsiveLayout } from "./useResponsiveLayout";

export function Player(
  props: Partial<PlayerProps> & {
    setFullscreen?: (fullscreen: boolean) => void;
  },
) {
  const [showChat, setShowChat] = useState(true);
  const { shouldShowChatSidePanel, chatPanelWidth, safeAreaInsets } =
    useResponsiveLayout();
  const chatVisible = shouldShowChatSidePanel && showChat;

  const [isStreamingElsewhere, setIsStreamingElsewhere] = useState<
    boolean | null
  >(null);
  // are we currently streaming on another device?
  const userIsLive = useLiveUser();
  const userProfile = useAppSelector(selectUserProfile);

  useEffect(() => {
    if (props.ingest && userIsLive && isStreamingElsewhere === null) {
      setIsStreamingElsewhere(true);
    } else if (props.ingest && userIsLive === false) {
      setIsStreamingElsewhere(false);
    }
  }, [userIsLive]);

  const navigation = useNavigation();

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
    <LivestreamProvider src={props.src ?? ""}>
      <PlayerProvider defaultId={props.playerId || undefined}>
        <View
          style={{
            flexDirection: chatVisible ? "row" : "column",
            flex: 1,
            width: "100%",
            height: "100%",
            paddingLeft: safeAreaInsets.left,
            paddingRight: safeAreaInsets.right,
          }}
        >
          <PlayerInner
            {...props}
            showChat={showChat}
            setShowChat={setShowChat}
          />
          {shouldShowChatSidePanel ? (
            <DesktopChatPanel
              chatVisible={chatVisible}
              chatPanelWidth={chatPanelWidth}
              safeAreaInsets={safeAreaInsets}
            />
          ) : (
            <MobileUi />
          )}
        </View>
      </PlayerProvider>
    </LivestreamProvider>
  );
}

export function PlayerInner(
  props: Partial<PlayerProps> & {
    showChat: boolean;
    setShowChat: (show: boolean) => void;
  },
) {
  let sb = useSidebarControl();
  let fullscreen = usePlayerStore((x) => x.fullscreen);
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

  // content info
  const { width, height } = usePlayerDimensions();

  // Calculate aspect ratio and determine if we're in desktop mode
  const aspectRatio = width > 0 && height > 0 ? width / height : 16 / 9;
  const isDesktopMode = shouldShowChatSidePanel || screenWidth > 768;

  // Calculate optimal height for desktop mode (90% of available height)
  const maxDesktopHeight = availableHeight * 0.8;
  const chatVisible = shouldShowChatSidePanel && props.showChat;

  const calculatedWidth = chatVisible
    ? contentWidth - chatPanelWidth
    : contentWidth;

  const calculatedHeight = isDesktopMode
    ? Math.min(calculatedWidth / aspectRatio, maxDesktopHeight)
    : height;

  const showBottomMetaPanel = aspectRatio > 1 && screenWidth > 980;

  // Direct responsive styling without animations
  const playerStyle = {
    width: calculatedWidth,
    height: calculatedHeight,
  };
  // i don't really like this, but it's the only way to ensure the
  // player is sized correctly on both desktop and mobile views
  const ContainerElement = showBottomMetaPanel ? ScrollView : View;
  return (
    <ContainerElement
      style={
        shouldShowChatSidePanel
          ? {
              height: "100%",
              width: calculatedWidth, // Add explicit width
            }
          : {
              height: "100%",
              flex: 1,
            }
      }
      contentContainerStyle={{
        width: calculatedWidth,
      }}
      showsVerticalScrollIndicator={false}
      bounces={false}
    >
      <Animated.View
        style={[
          showBottomMetaPanel
            ? {
                width: calculatedWidth,
                height: calculatedHeight,
              }
            : {
                flex: 1,
              },
        ]}
      >
        <PlayerInnerInner {...props}>
          {(showBottomMetaPanel || fullscreen) && <DesktopUi />}
          <OfflineCounter isMobile={true} />
        </PlayerInnerInner>
      </Animated.View>
      {showBottomMetaPanel && (
        <BottomMetadata
          setShowChat={props.setShowChat}
          showChat={props.showChat}
        />
      )}
    </ContainerElement>
  );
}
