import {
  Chat,
  ChatBox,
  IconButton,
  Loader,
  Resizable,
  StreamNotificationProvider,
  Text,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
import { scrims, spacing } from "@streamplace/components/src/lib/theme/tokens";
import { useKeyboard } from "hooks/useKeyboard";
import { useEffect, useState } from "react";
import { Platform, Pressable, useWindowDimensions } from "react-native";
import Animated, {
  SharedValue,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";

import { useNavigation } from "@react-navigation/native";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { BadgePicker } from "components/mobile/badge-picker";
import { ArrowRight, ChevronsRight } from "lucide-react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useStore } from "store";
import { useEmojiData } from "utils/emoji";
const { borderRadius, gap, layout, flex, px, position, bottom } = zero;

export function DesktopChatPanel({ chatVisible, chatPanelWidth, setShowChat }) {
  const { theme } = useTheme();
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
            backgroundColor: theme.colors.surface0,
            borderLeftWidth: 1,
            borderLeftColor: theme.colors.borderSubtle,
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
          <ChatPanel setShowChat={setShowChat} />
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
      kb.keyboardHeight > 0 ? scrims.dark : "rgba(0, 0, 0, 0)", // token-ok
      SpringSettings,
    ),
    borderRadius: withSpring(containerHeight > 0 ? 18 : 0, SpringSettings),
    transform: [
      {
        translateY:
          Platform.OS === "web"
            ? 0
            : withSpring(-kb.keyboardHeight, SpringSettings),
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
        <View
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            right: 0,
            zIndex: 2,
          }}
        >
          <StreamNotificationProvider position="top" />
        </View>
      </Animated.View>
    </View>
  );
}

// MobileChatPanel.tsx
export function MobileChatPanel({
  isPlayerRatioGreater,
  portraitVideoTranslateY,
  fixed = false,
}: {
  isPlayerRatioGreater: boolean;
  portraitVideoTranslateY?: SharedValue<number>;
  fixed?: boolean;
}) {
  const insets = useSafeAreaInsets();

  // create fixed style
  const fixedStyle = useAnimatedStyle(() => ({
    marginTop: portraitVideoTranslateY ? portraitVideoTranslateY.value : 0,
  }));

  if (fixed) {
    return (
      <Animated.View
        style={[
          {
            flex: 1,
            width: "100%",
            paddingBottom: insets.bottom,
            position: "relative",
          },
          fixedStyle,
        ]}
      >
        <FixedChatPanel />
      </Animated.View>
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

function ChatPanel({ setShowChat }: { setShowChat?: (show: boolean) => void }) {
  const { theme } = useTheme();
  let agent = usePDSAgent();

  const navigation = useNavigation();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const emojiData = useEmojiData();
  const customEmoji: any[] = [];

  return (
    <View
      style={[
        layout.flex.column,
        flex.values[1],
        { width: "100%", maxWidth: "100%", position: "relative" },
      ]}
    >
      {/* Panel header — labels the surface, hosts the collapse control */}
      {setShowChat && (
        <View
          style={[
            layout.flex.row,
            layout.flex.spaceBetween,
            layout.flex.alignCenter,
            {
              height: 48,
              paddingLeft: spacing[4],
              paddingRight: spacing[2],
              borderBottomWidth: 1,
              borderBottomColor: theme.colors.borderSubtle,
            },
          ]}
        >
          <Text weight="medium">Chat</Text>
          <IconButton
            size="sm"
            accessibilityLabel="Collapse chat"
            onPress={() => setShowChat(false)}
          >
            <ChevronsRight size={16} color={theme.colors.text2} />
          </IconButton>
        </View>
      )}
      <View style={[flex.values[1], px[2]]}>
        <Chat />
      </View>
      <View
        style={[
          layout.flex.column,
          gap.all[2],
          px[2],
          { paddingBottom: spacing[2] },
        ]}
      >
        {agent?.did ? (
          <ChatBox
            emojiData={emojiData}
            chatBoxStyle={{ borderRadius: borderRadius.lg }}
            setIsChatVisible={setShowChat ? (v) => setShowChat(v) : undefined}
            leftSlot={<BadgePicker />}
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
                borderRadius: borderRadius.lg,
                backgroundColor: theme.colors.surface2,
                borderWidth: 1,
                borderColor: theme.colors.borderSubtle,
              },
            ]}
          >
            <Loader size="large" />
          </View>
        ) : (
          <Pressable
            onPress={() => {
              const state = navigation.getState();
              if (!state) {
                openLoginModal();
                return;
              }
              let route: any = state.routes[state.index];
              while (route.state?.index !== undefined) {
                route = route.state.routes[route.state.index];
              }
              openLoginModal({ name: route.name, params: route.params } as any);
            }}
            style={[
              layout.flex.row,
              layout.flex.center,
              gap.all[2],
              {
                paddingVertical: spacing[2],
                paddingHorizontal: spacing[3],
                borderRadius: borderRadius.lg,
                backgroundColor: theme.colors.surface2,
                borderWidth: 1,
                borderColor: theme.colors.borderSubtle,
              },
            ]}
          >
            <Text size="sm">Log in or sign up to chat</Text>
            <ArrowRight color={theme.colors.text1} size={16} />
          </Pressable>
        )}
      </View>
    </View>
  );
}
