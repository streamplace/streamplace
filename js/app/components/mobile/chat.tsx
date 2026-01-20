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
import { useEffect } from "react";
import { Pressable } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";

import { useNavigation, useNavigationState } from "@react-navigation/native";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
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
        { width: "100%", maxWidth: "100%" },
        px[2],
      ]}
    >
      <View style={[flex.values[1]]}>
        <Chat />
      </View>
      <View style={[layout.flex.column, gap.all[2]]}>
        {/*
        // in case one needs this again

          <Pressable
          onPress={() =>
            StreamNotifications.activate("Stream notification activated!")
          }
          style={[
            layout.flex.row,
            layout.flex.center,
            {
              padding: 12,
              borderRadius: borderRadius.xl,
              backgroundColor: "rgba(255, 255, 255, 0.05)",
            },
          ]}
        >
          <Text style={{ color: "rgba(255, 255, 255, 0.7)", fontSize: 12 }}>
            Activate Stream Notification
          </Text>
        </Pressable>*/}
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
