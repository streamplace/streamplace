import {
  Chat,
  ChatBox,
  Resizable,
  Text,
  useHandle,
  useLivestreamInfo,
  View,
  zero,
} from "@streamplace/components";
import { useKeyboard } from "hooks/useKeyboard";
import { useEffect } from "react";
import { Keyboard, Pressable, TouchableWithoutFeedback } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { useResponsiveLayout } from "./useResponsiveLayout";

import { useNavigation } from "@react-navigation/native";
import { ArrowRight } from "@tamagui/lucide-icons";
import emojiData from "assets/emoji-data.json";
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
  const navigation = useNavigation();
  let canModerate = profile?.handle === handle;
  return (
    <View
      style={[
        layout.flex.column,
        flex.shrink[1],
        { width: "100%", maxWidth: "100%" },
      ]}
    >
      <TouchableWithoutFeedback onPress={Keyboard.dismiss} accessible={false}>
        <Chat canModerate={canModerate} />
      </TouchableWithoutFeedback>
      <View style={[layout.flex.column, gap.all[2], px[4]]}>
        {handle ? (
          <ChatBox
            emojiData={emojiData}
            chatBoxStyle={{ borderRadius: borderRadius.xl }}
          />
        ) : (
          <Pressable
            onPress={() => navigation.navigate("Login")}
            style={[
              layout.flex.row,
              layout.flex.center,
              gap.all[2],
              {
                padding: 16,
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
