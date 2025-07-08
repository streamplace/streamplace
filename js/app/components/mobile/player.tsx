import {
  LivestreamProvider,
  Player as PlayerInnerInner,
  PlayerProps,
  PlayerProvider,
  usePlayerDimensions,
  View,
} from "@streamplace/components";
import { useSidebarControl } from "hooks/useSidebarControl";
import { useState } from "react";
import { Animated, ScrollView } from "react-native";
import { BottomMetadata } from "./bottom-metadata";
import { DesktopChatPanel } from "./chat";
import { DesktopUi } from "./desktop-ui";
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

  return (
    <LivestreamProvider src={props.src ?? ""}>
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
  const {
    shouldShowChatSidePanel,
    chatPanelWidth,
    screenWidth,
    contentWidth,
    screenHeight,
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

  // Calculate optimal height for desktop mode (90% of screen height)
  const maxDesktopHeight = screenHeight * 0.8;
  const chatVisible = shouldShowChatSidePanel && props.showChat;
  const calculatedWidth = chatVisible
    ? contentWidth - chatPanelWidth
    : contentWidth;
  const calculatedHeight = isDesktopMode
    ? Math.min(calculatedWidth / aspectRatio, maxDesktopHeight)
    : height;

  // Direct responsive styling without animations
  const playerStyle = {
    width: calculatedWidth,
    height: calculatedHeight,
  };
  // i don't really like this, but it's the only way to ensure the
  // player is sized correctly on both desktop and mobile views
  const ContainerElement = shouldShowChatSidePanel ? ScrollView : View;
  return (
    <ContainerElement
      style={
        shouldShowChatSidePanel
          ? {
              height: "100%",
              width: calculatedWidth, // Add explicit width
            }
          : {
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
          shouldShowChatSidePanel
            ? {
                width: calculatedWidth,
                height: calculatedHeight,
              }
            : {
                flex: 1,
              },
        ]}
      >
        <PlayerInnerInner {...props} />
        {aspectRatio > 1 && <DesktopUi />}
      </Animated.View>
      {aspectRatio > 1 && (
        <BottomMetadata
          setShowChat={props.setShowChat}
          showChat={props.showChat}
        />
      )}
    </ContainerElement>
  );
}
