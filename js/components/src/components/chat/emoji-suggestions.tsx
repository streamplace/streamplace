import { useEffect, useRef } from "react";
import { Platform, Pressable } from "react-native";
import { ScrollView } from "react-native-gesture-handler";
import { Code, Text, View } from "../..";
import { bg, layout, left, right, zIndex } from "../../lib/theme/atoms";

export interface EmojiData {
  emojis: { [key: string]: Emoji };
  aliases: { [key: string]: string };
}

export interface Emoji {
  id: string;
  m: string;
  k: string[];
  s: Skin[];
}

export interface Skin {
  n: string;
}

interface EmojiSuggestionsProps {
  emojis: Emoji[];
  onSelect: (emoji: Emoji) => void;
  highlightedIndex: number;
  skinTone?: number;
}

export function getSkinNative(emoji: Emoji, skinTone: number = 0): string {
  return (emoji.s[skinTone] ?? emoji.s[0]).n;
}

export function EmojiSuggestions({
  emojis,
  onSelect,
  highlightedIndex,
  skinTone = 0,
}: EmojiSuggestionsProps) {
  if (!emojis || emojis.length === 0) {
    return null;
  }

  const itemRefs = useRef<Map<number, HTMLElement>>(new Map());

  useEffect(() => {
    if (Platform.OS === "web") {
      const el = itemRefs.current.get(highlightedIndex);
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "nearest" });
      }
    }
  }, [highlightedIndex]);

  return (
    <View
      style={[
        bg.gray[800],
        layout.position.absolute,
        left[0],
        right[0],
        zIndex[50],
        {
          bottom: "100%",
          borderRadius: 8,
          boxShadow: "0px 4px 6px rgba(0, 0, 0, 0.1)",
          maxHeight: 200,
        },
      ]}
    >
      <ScrollView>
        {emojis.map((emoji, index) => (
          <Pressable
            key={emoji.id}
            onPress={() => onSelect(emoji)}
            ref={(ref) => {
              const el = ref as unknown as HTMLElement;
              if (el) itemRefs.current.set(index, el);
            }}
            style={[
              {
                padding: 8,
                flexDirection: "row",
                alignItems: "center",
                backgroundColor:
                  index === highlightedIndex
                    ? "rgba(255, 255, 255, 0.1)"
                    : "transparent",
              },
            ]}
          >
            <Text style={{ fontSize: 16, marginRight: 8 }}>
              {getSkinNative(emoji, skinTone)}
            </Text>
            <Text style={{ color: "white", fontSize: 14 }}>
              <Code style={[bg.gray[950]]}>:{emoji.id}:</Code> {emoji.m}
            </Text>
          </Pressable>
        ))}
      </ScrollView>
    </View>
  );
}
