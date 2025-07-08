import {
  Chat,
  ChatBox,
  Resizable,
  useLivestreamInfo,
  View,
  zero,
} from "@streamplace/components";
import { useKeyboard } from "hooks/useKeyboard";
import { useEffect } from "react";
import { Keyboard, TouchableWithoutFeedback } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { borderRadius, gap, layout, flex, px, py, r, position, bottom } = zero;

export function DesktopChatPanel({
  chatVisible,
  chatPanelWidth,
  safeAreaInsets,
}) {
  const sidebarOffset = useSharedValue(chatVisible ? 0 : chatPanelWidth);

  const kb = useKeyboard();

  useEffect(() => {
    console.log(
      "Setting sidebar offset x to",
      chatVisible ? 0 : chatPanelWidth,
    );
    sidebarOffset.value = withSpring(chatVisible ? 0 : chatPanelWidth, {
      damping: 100,
      stiffness: 1000,
    });
  }, [chatVisible, chatPanelWidth, sidebarOffset]);

  const animatedSidebarStyle = useAnimatedStyle(() => ({
    transform: [
      { translateX: sidebarOffset.value },
      { translateY: -kb.keyboardHeight },
    ],
  }));

  return (
    <TouchableWithoutFeedback onPress={Keyboard.dismiss} accessible={false}>
      <Animated.View
        style={[
          layout.position.absolute,
          position.right[0],
          {
            top: safeAreaInsets.top,
            bottom: safeAreaInsets.bottom,
            right: safeAreaInsets.right / 2,
            width: chatPanelWidth,
            backgroundColor: "rgba(0, 0, 0, 0.85)",
            borderLeftWidth: 1,
            borderLeftColor: "rgba(255, 255, 255, 0.1)",
            zIndex: 999,
          },
          animatedSidebarStyle,
        ]}
      >
        <View style={{ flex: 1, position: "relative" }}>
          <ChatPanel />
        </View>
      </Animated.View>
    </TouchableWithoutFeedback>
  );
}

// MobileChatPanel.tsx
export function MobileChatPanel({ isPlayerRatioGreater }) {
  return (
    <View
      style={[
        isPlayerRatioGreater
          ? layout.position.relative
          : layout.position.absolute,
        bottom[0],
        { width: "100%", maxWidth: "100%" },
      ]}
    >
      <Resizable
        isPlayerRatioGreater={isPlayerRatioGreater}
        startingPercentage={0.4}
      >
        <ChatPanel />
      </Resizable>
    </View>
  );
}

function ChatPanel() {
  const { shouldShowChatSidePanel, safeAreaInsets } = useResponsiveLayout();
  const { profile } = useLivestreamInfo();
  return (
    <View
      style={[
        layout.flex.column,
        flex.shrink[1],
        {
          width: "100%",
          maxWidth: "100%",
          height: "100%",
          justifyContent: "flex-end",
          ...(shouldShowChatSidePanel && {
            paddingTop: shouldShowChatSidePanel ? 0 : safeAreaInsets.top, // Account for top UI elements and safe area
          }),
        },
        ...(shouldShowChatSidePanel ? [px[4]] : []),
      ]}
    >
      <Chat canModerate={false} />
      <View style={[layout.flex.column, gap.all[2], px[4]]}>
        <ChatBox chatBoxStyle={{ borderRadius: borderRadius.xl }} />
      </View>
    </View>
  );
}
