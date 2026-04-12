import { Text, useTheme, zero } from "@streamplace/components";
import { Image } from "expo-image";
import { useKeyboard } from "hooks/useKeyboard";
import { useEffect, useRef } from "react";
import { FlatList, Keyboard, Pressable, View } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
  withTiming,
} from "react-native-reanimated";

const { bg, borders, gap, layout, p, py, r, text } = zero;

export type SelectedEmoji =
  | { type: "standard"; native: string }
  | { type: "custom"; name: string; aturi: string; cid: string };

export interface CustomEmojiEntry {
  name: string;
  imageUrl: string;
  aturi: string;
  cid: string;
  alt?: string;
}

export interface EmojiPack {
  name: string;
  emoji: CustomEmojiEntry[];
}

interface EmojiPickerProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect?: (emoji: SelectedEmoji) => void;
  emojiPacks?: EmojiPack[];
}

const PANEL_HEIGHT = 265;

export function EmojiPicker({
  isOpen,
  onClose,
  onSelect,
  emojiPacks = [],
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
    height.value = isOpen
      ? withSpring(PANEL_HEIGHT, { damping: 30, stiffness: 300 })
      : withTiming(0, { duration: 200 });
  }, [isOpen, kb.isKeyboardVisible]);

  const animatedStyle = useAnimatedStyle(() => ({
    height: height.value,
    overflow: "hidden",
  }));

  const handleSelect = (item: CustomEmojiEntry) => {
    onSelect?.({
      type: "custom",
      name: item.name,
      aturi: item.aturi,
      cid: item.cid,
    });
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
        {emojiPacks.length === 0 ? (
          <Animated.View
            style={[layout.flex.column, layout.flex.center, { flex: 1 }]}
          >
            <Text style={[text.gray[600]]}>No custom emoji available</Text>
          </Animated.View>
        ) : (
          emojiPacks.map((pack) => (
            <View key={pack.name}>
              <Text>{pack.name}</Text>
              <FlatList
                data={pack.emoji}
                keyExtractor={(item) => item.name}
                numColumns={8}
                renderItem={({ item }) => (
                  <Pressable
                    onPress={() => handleSelect(item)}
                    style={({ pressed }) => ({
                      width: 36,
                      height: 36,
                      margin: 2,
                      borderRadius: 6,
                      padding: 3,
                      backgroundColor: pressed
                        ? theme.colors.muted
                        : "transparent",
                      alignItems: "center",
                      justifyContent: "center",
                    })}
                  >
                    <Image
                      source={{ uri: item.imageUrl }}
                      style={{ width: 28, height: 28 }}
                      resizeMode="contain"
                      accessibilityLabel={item.alt ?? item.name}
                    />
                  </Pressable>
                )}
              />
            </View>
          ))
        )}
      </Animated.View>
    </Animated.View>
  );
}
