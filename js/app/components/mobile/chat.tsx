import {
  Chat,
  ChatBox,
  Loader,
  Resizable,
  Text,
  useHandle,
  useLivestreamInfo,
  View,
  zero,
} from "@streamplace/components";
import { useKeyboard } from "hooks/useKeyboard";
import { useEffect } from "react";
import { Pressable } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { useResponsiveLayout } from "./useResponsiveLayout";

import { useNavigation } from "@react-navigation/native";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { ArrowRight } from "@tamagui/lucide-icons";
import emojiData from "assets/emoji-data.json";
const { borderRadius, gap, layout, flex, px, position, bottom } = zero;

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
  const handle = useHandle();

  let agent = usePDSAgent();

  const navigation = useNavigation();
  let canModerate = profile?.handle === handle;

  return (
    <View
      style={[
        layout.flex.column,
        flex.values[1],
        { width: "100%", maxWidth: "100%" },
      ]}
    >
      <View style={[flex.values[1]]}>
        <Chat canModerate={canModerate} />
      </View>
      <View style={[layout.flex.column, gap.all[2], px[4]]}>
        {agent?.did ? (
          <ChatBox
            emojiData={emojiData}
            chatBoxStyle={{ borderRadius: borderRadius.xl }}
          />
        ) : !agent ? (
          <View
            style={[
              layout.flex.row,
              layout.flex.center,
              gap.all[1],
              zero.p[3],
              {
                borderRadius: borderRadius.xl,
                backgroundColor: "rgba(255, 255, 255, 0.1)",
              },
            ]}
          >
            <Loader size="large" />
          </View>
        ) : (
          <Pressable
            onPress={() => navigation.navigate("Login")}
            style={[
              layout.flex.row,
              layout.flex.center,
              gap.all[2],
              {
                padding: 18,
                borderRadius: borderRadius.xl,
                backgroundColor: "rgba(255, 255, 255, 0.1)",
              },
            ]}
          >
            <Text>Log in or sign up to chat</Text>
            <ArrowRight />
          </Pressable>
        )}
      </View>
    </View>
  );
}
