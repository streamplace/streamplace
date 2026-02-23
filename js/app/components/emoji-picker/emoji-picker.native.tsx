import { useEffect } from "react";
import { FlatList, Image, Pressable, Text, View } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { emojiEmitter } from "./emoji-emitter";

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
  customEmoji?: CustomEmojiEntry[];
}

const PANEL_HEIGHT = 260;

export function EmojiPicker({
  isOpen,
  onClose,
  customEmoji = [],
}: EmojiPickerProps) {
  const translateY = useSharedValue(PANEL_HEIGHT);

  useEffect(() => {
    translateY.value = withSpring(isOpen ? 0 : PANEL_HEIGHT, {
      damping: 30,
      stiffness: 300,
    });
  }, [isOpen]);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ translateY: translateY.value }],
  }));

  if (!isOpen && translateY.value === PANEL_HEIGHT) return null;

  const handleSelect = (name: string) => {
    emojiEmitter.emit("emoji-selected", {
      type: "custom",
      name,
    } satisfies SelectedEmoji);
    onClose();
  };

  return (
    <>
      <Pressable
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 100,
        }}
        onPress={onClose}
      />
      <Animated.View
        style={[
          {
            position: "absolute",
            bottom: 0,
            left: 0,
            right: 0,
            height: PANEL_HEIGHT,
            backgroundColor: "#1f2937",
            borderTopWidth: 1,
            borderTopColor: "rgba(255,255,255,0.1)",
            zIndex: 101,
            borderTopLeftRadius: 16,
            borderTopRightRadius: 16,
            padding: 12,
          },
          animatedStyle,
        ]}
      >
        <Text
          style={{
            fontSize: 11,
            fontWeight: "600",
            color: "rgba(255,255,255,0.4)",
            textTransform: "uppercase",
            letterSpacing: 0.8,
            marginBottom: 8,
          }}
        >
          Custom Emoji
        </Text>
        {customEmoji.length === 0 ? (
          <View
            style={{ flex: 1, alignItems: "center", justifyContent: "center" }}
          >
            <Text style={{ color: "rgba(255,255,255,0.3)", fontSize: 13 }}>
              No custom emoji available
            </Text>
          </View>
        ) : (
          <FlatList
            data={customEmoji}
            keyExtractor={(item) => item.name}
            numColumns={8}
            renderItem={({ item }) => (
              <Pressable
                onPress={() => handleSelect(item.name)}
                style={({ pressed }) => ({
                  width: 40,
                  height: 40,
                  margin: 2,
                  borderRadius: 6,
                  padding: 4,
                  backgroundColor: pressed
                    ? "rgba(255,255,255,0.15)"
                    : "transparent",
                  alignItems: "center",
                  justifyContent: "center",
                })}
              >
                <Image
                  source={{ uri: item.imageUrl }}
                  style={{ width: 32, height: 32 }}
                  resizeMode="contain"
                  accessibilityLabel={item.alt ?? item.name}
                />
              </Pressable>
            )}
          />
        )}
      </Animated.View>
    </>
  );
}
