import { ChevronDown, Ellipsis, Reply } from "lucide-react-native";
import {
  ComponentProps,
  memo,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Keyboard, Platform, Pressable } from "react-native";
import { FlatList } from "react-native-gesture-handler";
import Swipeable, {
  SwipeableMethods,
} from "react-native-gesture-handler/ReanimatedSwipeable";
import Reanimated, {
  SharedValue,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { ChatMessageViewHydrated } from "streamplace";
import {
  ErrorBoundary,
  getSystemMessageType,
  Skeleton,
  SystemMessage,
  SystemMessageType,
  Text,
  useChat,
  usePlayerStore,
  useSetReplyToMessage,
  useTheme,
  View,
} from "../../";
import { flex, gap, layout, mr, px, py } from "../../lib/theme/atoms";
import { borderRadius, colors, motion, spacing } from "../../lib/theme/tokens";
import { RenderChatMessage } from "./chat-message";
import { ModView } from "./mod-view";
import { ProfileCardProvider } from "./user-profile-card";

function RightAction(prog: SharedValue<number>, drag: SharedValue<number>) {
  const styleAnimation = useAnimatedStyle(() => {
    return {
      transform: [{ translateX: drag.value + 25 }],
    };
  });

  return (
    <Reanimated.View style={[styleAnimation]}>
      <Reply color={colors.white} />
    </Reanimated.View>
  );
}

function LeftAction(prog: SharedValue<number>, drag: SharedValue<number>) {
  const styleAnimation = useAnimatedStyle(() => {
    return {
      transform: [{ translateX: drag.value - 25 }],
    };
  });

  return (
    <Reanimated.View style={[styleAnimation]}>
      <Ellipsis color={colors.white} />
    </Reanimated.View>
  );
}

// ios/android, 25, else 100 msgs
const SHOWN_MSGS =
  Platform.OS === "ios" || Platform.OS === "android" ? 25 : 100;

const keyExtractor = (item: ChatMessageViewHydrated, index: number) => {
  return `${item.uri}`;
};

// Actions bar for larger screens
const ActionsBar = memo(
  ({
    item,
    visible,
    hoverTimeoutRef,
  }: {
    item: ChatMessageViewHydrated;
    visible: boolean;
    hoverTimeoutRef: React.MutableRefObject<NodeJS.Timeout | null>;
  }) => {
    const setReply = useSetReplyToMessage();
    const setModMsg = usePlayerStore((state) => state.setModMessage);
    const { theme } = useTheme();

    if (!visible) return null;

    return (
      <View
        style={[
          {
            position: "absolute",
            top: -14,
            right: 8,
            flexDirection: "row",
            backgroundColor: theme.colors.surface3,
            borderRadius: borderRadius.sm,
            borderWidth: 1,
            borderColor: theme.colors.borderStrong,
            padding: 1,
            gap: spacing[1],
            zIndex: 10,
            maxWidth: 120,
            flexShrink: 0,
          },
        ]}
      >
        <Pressable
          onPress={() => setReply(item)}
          style={[
            {
              padding: 6,
              borderRadius: borderRadius.sm,
            },
          ]}
          onHoverIn={() => {
            // Keep the actions bar visible when hovering over it
            if (hoverTimeoutRef.current) {
              clearTimeout(hoverTimeoutRef.current);
              hoverTimeoutRef.current = null;
            }
          }}
        >
          <Reply color={theme.colors.text2} size={16} />
        </Pressable>
        <Pressable
          onPress={() => setModMsg(item)}
          style={[
            {
              padding: 6,
              borderRadius: borderRadius.sm,
            },
          ]}
          onHoverIn={() => {
            // Keep the actions bar visible when hovering over it
            if (hoverTimeoutRef.current) {
              clearTimeout(hoverTimeoutRef.current);
              hoverTimeoutRef.current = null;
            }
          }}
        >
          <Ellipsis color={theme.colors.text2} size={16} />
        </Pressable>
      </View>
    );
  },
);

const ChatLine = memo(({ item }: { item: ChatMessageViewHydrated }) => {
  const { theme } = useTheme();
  const setReply = useSetReplyToMessage();
  const setModMsg = usePlayerStore((state) => state.setModMessage);
  const swipeableRef = useRef<SwipeableMethods | null>(null);
  const [isHovered, setIsHovered] = useState(false);
  const hoverTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const handleHoverIn = () => {
    if (hoverTimeoutRef.current) {
      clearTimeout(hoverTimeoutRef.current);
      hoverTimeoutRef.current = null;
    }
    setIsHovered(true);
  };

  const handleHoverOut = () => {
    hoverTimeoutRef.current = setTimeout(() => {
      setIsHovered(false);
    }, 50);
  };

  useEffect(() => {
    return () => {
      if (hoverTimeoutRef.current) {
        clearTimeout(hoverTimeoutRef.current);
      }
    };
  }, []);

  if (item.author.did === "did:sys:system") {
    return (
      <SystemMessage
        variant={getSystemMessageType(item) || SystemMessageType.notification}
        timestamp={new Date(item.record.createdAt)}
        title={item.record.text}
        facets={item.record.facets}
      />
    );
  }

  if (Platform.OS === "web") {
    return (
      <View
        style={[
          py[1],
          px[2],
          {
            position: "relative",
            borderRadius: borderRadius.md,
            minWidth: 0,
            maxWidth: "100%",
          },
          isHovered ? { backgroundColor: theme.colors.surfaceHover } : {},
        ]}
        onPointerEnter={handleHoverIn}
        onPointerLeave={handleHoverOut}
      >
        <Pressable style={[{ minWidth: 0, maxWidth: "100%" }]}>
          <RenderChatMessage item={item} />
        </Pressable>
        <ActionsBar
          item={item}
          visible={isHovered}
          hoverTimeoutRef={hoverTimeoutRef}
        />
      </View>
    );
  }

  return (
    <>
      <Swipeable
        containerStyle={[{ paddingVertical: 6 }]}
        friction={2}
        enableTrackpadTwoFingerGesture
        rightThreshold={40}
        leftThreshold={40}
        renderRightActions={Platform.OS === "android" ? undefined : RightAction}
        renderLeftActions={Platform.OS === "android" ? undefined : LeftAction}
        overshootFriction={9}
        ref={swipeableRef}
        onSwipeableOpen={(r) => {
          if (r === (Platform.OS === "android" ? "right" : "left")) {
            setReply(item);
          }
          if (r === (Platform.OS === "android" ? "left" : "right")) {
            setModMsg(item);
          }
          // close this swipeable
          const swipeable = swipeableRef.current;
          if (swipeable) {
            swipeable.close();
          }
        }}
      >
        <RenderChatMessage item={item} />
      </Swipeable>
    </>
  );
});

export function Chat({
  shownMessages = SHOWN_MSGS,
  style: propsStyle,
  reverse = false,
  hideAfter,
  ...props
}: ComponentProps<typeof View> & {
  shownMessages?: number;
  style?: ComponentProps<typeof View>["style"];
  reverse?: boolean;
  hideAfter?: number;
}) {
  const { theme } = useTheme();
  const chat = useChat();
  const [isScrolledUp, setIsScrolledUp] = useState(false);
  const [isVisible, setIsVisible] = useState(true);
  const flatListRef = useRef<FlatList>(null);
  // The store keeps chat oldest-first. An inverted FlatList renders index 0 at
  // the bottom, so feed it newest-first to keep the latest message at the
  // bottom (or at the top when reverse is set, where inverted is off).
  const displayMessages = useMemo(
    () => (chat ? chat.slice(-shownMessages).reverse() : []),
    [chat, shownMessages],
  );
  const latestMessageTime = displayMessages[0]
    ? new Date(displayMessages[0].record.createdAt).getTime()
    : null;

  // Animation for scroll-to-bottom button
  const buttonOpacity = useSharedValue(0);
  const buttonTranslateY = useSharedValue(20);

  useEffect(() => {
    buttonOpacity.value = withTiming(isScrolledUp ? 1 : 0, {
      duration: motion.base,
    });
    buttonTranslateY.value = withTiming(isScrolledUp ? 0 : 50, {
      duration: motion.base,
    });
  }, [isScrolledUp]);

  const buttonAnimatedStyle = useAnimatedStyle(() => ({
    opacity: buttonOpacity.value,
    transform: [{ translateY: buttonTranslateY.value }],
  }));

  const scrollToLatest = () => {
    flatListRef.current?.scrollToOffset({ offset: 0, animated: true });
  };

  const handleScroll = (event: any) => {
    const { contentOffset } = event.nativeEvent;

    const scrolledUp = contentOffset.y > 20; // threshold

    if (scrolledUp !== isScrolledUp) {
      setIsScrolledUp(scrolledUp);

      // Dismiss keyboard when scrolled up
      if (scrolledUp && Platform.OS !== "web") {
        Keyboard.dismiss();
      }
    }
  };

  useEffect(() => {
    if (!hideAfter || hideAfter <= 0) return;

    const referenceTime = latestMessageTime ?? Date.now();
    const delay = referenceTime + hideAfter * 1000 - Date.now();

    if (delay <= 0) {
      setIsVisible(false);
      return;
    }

    setIsVisible(true);
    const timer = setTimeout(() => {
      setIsVisible(false);
    }, delay);
    return () => clearTimeout(timer);
  }, [hideAfter, latestMessageTime]);

  if (!isVisible) return null;

  if (!chat)
    return (
      <View
        style={[
          flex.values[1],
          px[2],
          py[2],
          gap.all[3],
          { minWidth: 0, maxWidth: "100%", justifyContent: "flex-end" },
        ]}
      >
        {[78, 52, 88, 40, 64, 72].map((w, i) => (
          <Skeleton key={i} shape="text" width={`${w}%`} height={13} />
        ))}
      </View>
    );

  return (
    <View
      style={[
        flex.values[1],
        {
          minWidth: 0,
          maxWidth: "100%",
          position: "relative",
          overflow: "visible",
        },
      ].concat(propsStyle || [])}
    >
      <ProfileCardProvider>
        <FlatList
          ref={flatListRef}
          style={[
            flex.grow[1],
            flex.shrink[1],
            { minWidth: 0, maxWidth: "100%" },
          ]}
          data={displayMessages}
          inverted={!reverse}
          keyExtractor={keyExtractor}
          renderItem={({ item, index }) => (
            <ErrorBoundary>
              <ChatLine item={item} />
            </ErrorBoundary>
          )}
          removeClippedSubviews={true}
          maxToRenderPerBatch={10}
          initialNumToRender={10}
          updateCellsBatchingPeriod={50}
          onScroll={handleScroll}
          scrollEventThrottle={16}
          nestedScrollEnabled={true}
        />
      </ProfileCardProvider>
      <Reanimated.View
        style={[
          {
            position: "absolute",
            bottom: 16,
            left: 0,
            right: 0,
            alignItems: "center",
            pointerEvents: isScrolledUp ? "box-none" : "none",
          },
          buttonAnimatedStyle,
        ]}
      >
        <Pressable
          onPress={scrollToLatest}
          style={[
            {
              pointerEvents: isScrolledUp ? "auto" : "none",
              backgroundColor: theme.colors.surface3,
              borderWidth: 1,
              borderColor: theme.colors.borderStrong,
              borderRadius: borderRadius.full,
              ...theme.shadows.sm,
            },
            layout.flex.row,
            layout.flex.center,
            px[3],
            py[1],
            { gap: spacing[1] },
          ]}
        >
          <ChevronDown size={16} color={theme.colors.text1} />
          <Text size="sm" weight="medium" style={[mr[1]]}>
            {reverse ? "New messages above" : "New messages"}
          </Text>
        </Pressable>
      </Reanimated.View>
      <ModView />
    </View>
  );
}
