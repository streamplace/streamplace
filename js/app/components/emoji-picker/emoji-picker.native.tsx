import { Text, useTheme, zero } from "@streamplace/components";
import { Image } from "expo-image";
import { useKeyboard } from "hooks/useKeyboard";
import { useEffect, useRef } from "react";
import { FlatList, Keyboard, Pressable } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";

const { bg, borders, gap, layout, p, py, r, text } = zero;

export type SelectedEmoji =
  | { type: "standard"; native: string }
  | { type: "custom"; name: string };

export interface CustomEmojiEntry {
  name: string;
  imageUrl: string;
  alt?: string;
}

interface EmojiPickerProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect?: (emoji: SelectedEmoji) => void;
  customEmoji?: CustomEmojiEntry[];
}

const PANEL_HEIGHT = 265;

export function EmojiPicker({
  isOpen,
  onClose,
  onSelect,
  customEmoji = [],
}: EmojiPickerProps) {
  const { theme, zero: z } = useTheme();
  const height = useSharedValue(0);
  const kb = useKeyboard();
  const hasOpened = useRef(false);

  useEffect(() => {
    // if keyboard unexpectedly appears while emoji picker is open, close the picker
    if (kb.isKeyboardVisible && hasOpened.current === true) {
      hasOpened.current = false;
      onClose();
      return;
    }
    hasOpened.current = isOpen;
    // ensure the keyboard is dismissed when the emoji picker is opened
    kb.isKeyboardVisible && isOpen && Keyboard.dismiss();
    height.value = withSpring(isOpen ? PANEL_HEIGHT : 0, {
      damping: 30,
      stiffness: 300,
    });
  }, [isOpen, kb.isKeyboardVisible]);

  const animatedStyle = useAnimatedStyle(() => ({
    height: height.value,
    overflow: "hidden",
  }));

  const handleSelect = (name: string) => {
    onSelect?.({ type: "custom", name });
    onClose();
  };

  return (
    <Animated.View style={animatedStyle}>
      <Animated.View
        style={[
          z.bg.background,
          z.border.border,
          zero.borders.width.thin,
          r.xl,
          p[3],
          { height: PANEL_HEIGHT },
        ]}
      >
        <Text
          style={[
            text.gray[500],
            {
              fontSize: 10,
              fontWeight: "600",
              textTransform: "uppercase",
              letterSpacing: 0.8,
              marginBottom: 8,
            },
          ]}
        >
          Custom Emoji
        </Text>
        {customEmoji.length === 0 ? (
          <Animated.View
            style={[layout.flex.column, layout.flex.center, { flex: 1 }]}
          >
            <Text style={[text.gray[600]]}>No custom emoji available</Text>
          </Animated.View>
        ) : (
          <FlatList
            data={customEmoji}
            keyExtractor={(item) => item.name}
            numColumns={8}
            renderItem={({ item }) => (
              <Pressable
                onPress={() => handleSelect(item.name)}
                style={({ pressed }) => ({
                  width: 36,
                  height: 36,
                  margin: 2,
                  borderRadius: 6,
                  padding: 3,
                  backgroundColor: pressed ? theme.colors.muted : "transparent",
                  alignItems: "center",
                  justifyContent: "center",
                })}
              >
                <Image
                  source={{ uri: item.imageUrl }}
                  style={{ width: 28, height: 28 }}
                  contentFit="contain"
                  accessibilityLabel={item.alt ?? item.name}
                />
              </Pressable>
            )}
          />
        )}
      </Animated.View>
    </Animated.View>
  );
}
