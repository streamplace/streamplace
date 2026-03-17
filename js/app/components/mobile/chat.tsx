import {
  Chat,
  ChatBox,
  Loader,
  Resizable,
  StreamNotificationProvider,
  Text,
  useHandle,
  useLivestreamInfo,
  View,
  zero,
} from "@streamplace/components";
import { useKeyboard } from "hooks/useKeyboard";
import { useEffect, useState } from "react";
import { Pressable, useWindowDimensions } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";

import { useNavigation, useNavigationState } from "@react-navigation/native";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { ArrowRight } from "lucide-react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useStore } from "store";
import { useEmojiData } from "utils/emoji";
const { borderRadius, gap, layout, flex, px, position, bottom } = zero;

export function DesktopChatPanel({ chatVisible, chatPanelWidth }) {
  let insets = useSafeAreaInsets();
  let panelWidthWithInsets = chatPanelWidth;
  const sidebarOffset = useSharedValue(chatVisible ? 0 : panelWidthWithInsets);
  const sidebarOpacity = useSharedValue(chatVisible ? 1 : 0);

  const kb = useKeyboard();

  useEffect(() => {
    console.log(
      "Setting sidebar offset x to",
      chatVisible ? 0 : panelWidthWithInsets,
    );
    sidebarOffset.value = withSpring(chatVisible ? 0 : panelWidthWithInsets, {
      damping: 100,
      stiffness: 1000,
    });
    sidebarOpacity.value = withSpring(chatVisible ? 1 : 0, {
      damping: 100,
      stiffness: 1000,
    });
  }, [chatVisible, panelWidthWithInsets, sidebarOffset]);

  const animatedSidebarStyle = useAnimatedStyle(() => ({
    transform: [
      { translateX: sidebarOffset.value },
      { translateY: -kb.keyboardHeight },
    ],
  }));

  const notificationOffsetStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: -sidebarOffset.value }],
  }));

  return (
    <>
      <Animated.View
        style={[
          {
            width: chatPanelWidth,
            flexShrink: 0,
          },
          animatedSidebarStyle,
        ]}
      />
      <Animated.View
        style={[
          {
            position: "absolute",
            right: 0,
            // attempt to lessen the impact of the safe area inset on the chat panel?
            paddingRight: insets.right > 0 ? insets.right - 20 : 0,
            top: 0,
            bottom: 0,
            width: panelWidthWithInsets,
            flexShrink: 0,
            backgroundColor: "rgba(0, 0, 0, 0.85)",
            borderLeftWidth: 1,
            borderLeftColor: "rgba(255, 255, 255, 0.1)",
            zIndex: 999,
          },
          animatedSidebarStyle,
        ]}
      >
        <View style={{ flex: 1, position: "relative" }}>
          <Animated.View
            style={[
              {
                position: "absolute",
                top: 1,
                right: 1,
                zIndex: 2,
                width: "100%",
                minWidth: 350,
                pointerEvents: "none",
                transformOrigin: "top right",
              },
              notificationOffsetStyle,
            ]}
          >
            <StreamNotificationProvider position="top" />
          </Animated.View>
          <ChatPanel />
        </View>
      </Animated.View>
    </>
  );
}

function FixedChatPanel() {
  const kb = useKeyboard();
  const [containerHeight, setContainerHeight] = useState(0);
  const sa = useSafeAreaInsets();
  const dims = useWindowDimensions();

  console.log("container height", containerHeight);

  const SpringSettings = {
    duration: 25,
    mass: 10,
  };

  // calculate top bar + safe area height
  const videoHeight = dims.height - sa.top - 88 - containerHeight;
  const calculatedHeight =
    containerHeight + (kb.keyboardHeight > 0 ? videoHeight : 0);

  const animatedSidebarStyle = useAnimatedStyle(() => ({
    height: withSpring(
      containerHeight > 0
        ? Math.max(0, calculatedHeight - kb.keyboardHeight)
        : containerHeight,
      SpringSettings,
    ),
    backgroundColor: withSpring(
      `rgba(0, 0, 0, ${kb.keyboardHeight > 0 ? 0.5 : 0})`,
      SpringSettings,
    ),
    borderRadius: withSpring(containerHeight > 0 ? 18 : 0, SpringSettings),
    transform: [
      {
        translateY: withSpring(-kb.keyboardHeight, SpringSettings),
      },
    ],
  }));

  return (
    <View
      style={{ position: "relative", flex: 1 }}
      onLayout={(e) => setContainerHeight(e.nativeEvent.layout.height)}
    >
      <Animated.View
        style={[
          { position: "absolute", bottom: 0, left: 0, right: 0 },
          animatedSidebarStyle,
        ]}
      >
        <ChatPanel />
      </Animated.View>
    </View>
  );
}

// MobileChatPanel.tsx
export function MobileChatPanel({
  isPlayerRatioGreater,
  fixed = false,
}: {
  isPlayerRatioGreater: boolean;
  fixed?: boolean;
}) {
  const insets = useSafeAreaInsets();

  if (fixed) {
    return (
      <View
        style={{
          flex: 1,
          width: "100%",
          paddingBottom: insets.bottom,
          position: "relative",
        }}
      >
        <View
          style={{
            position: "absolute",
            bottom: 0,
            left: 0,
            right: 0,
            pointerEvents: "none",
          }}
        >
          <StreamNotificationProvider position="bottom" />
        </View>
        <FixedChatPanel />
      </View>
    );
  }

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
        renderAbove={(isCollapsed) => (
          <StreamNotificationProvider position="bottom" />
        )}
      >
        <ChatPanel />
      </Resizable>
    </View>
  );
}

function ChatPanel() {
  const { profile } = useLivestreamInfo();
  const handle = useHandle();

  let agent = usePDSAgent();

  const navigation = useNavigation();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const emojiData = useEmojiData();
  const customEmoji: any[] = [];

  // get the deepest active route for nested navigators
  const currentRoute = useNavigationState((state) => {
    let route: any = state.routes[state.index];
    while (route.state?.index !== undefined) {
      route = route.state.routes[route.state.index];
    }
    return { name: route.name, params: route.params };
  });

  return (
    <View
      style={[
        layout.flex.column,
        flex.values[1],
        { width: "100%", maxWidth: "100%", position: "relative" },
        px[2],
      ]}
    >
      <View style={[flex.values[1]]}>
        <Chat />
      </View>
      <View style={[layout.flex.column, gap.all[2]]}>
        {agent?.did ? (
          <ChatBox
            emojiData={emojiData}
            chatBoxStyle={{ borderRadius: borderRadius.xl }}
            emojiPicker={(isOpen, onClose, onSelect) => (
              <EmojiPicker
                isOpen={isOpen}
                onClose={onClose}
                onSelect={onSelect}
                customEmoji={customEmoji}
              />
            )}
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
            onPress={() => openLoginModal(currentRoute as any)}
            style={[
              layout.flex.row,
              layout.flex.center,
              gap.all[4],
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
