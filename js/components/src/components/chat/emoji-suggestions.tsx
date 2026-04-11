import { Image, Pressable } from "react-native";
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
        {emojis.map((emoji: any, index) => (
          <Pressable
            key={
              (emoji as any).type === "custom"
                ? `custom:${(emoji as any).name}`
                : emoji.id
            }
            onPress={() => onSelect(emoji)}
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
            {(emoji as any).type === "custom" ? (
              <Image
                source={{ uri: (emoji as any).imageUrl }}
                style={{ width: 20, height: 20, marginRight: 8 }}
              />
            ) : (
              <Text style={{ fontSize: 16, marginRight: 8 }}>
                {getSkinNative(emoji, skinTone)}
              </Text>
            )}
            <Text style={{ color: "white", fontSize: 14 }}>
              <Code style={[bg.gray[950]]}>
                :
                {(emoji as any).type === "custom"
                  ? (emoji as any).name
                  : emoji.id}
                :
              </Code>{" "}
              {(emoji as any).type !== "custom" && emoji.m}
            </Text>
          </Pressable>
        ))}
      </ScrollView>
    </View>
  );
}
