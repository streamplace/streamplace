import { Pressable } from "react-native";
import { Code, Text, View } from "../..";
import { bg, layout, left, right, zIndex } from "../../lib/theme/atoms";

export interface EmojiData {
  categories: Category[];
  emojis: { [key: string]: Emoji };
  aliases: { [key: string]: string };
  sheet: Sheet;
}

export interface Category {
  id: string;
  emojis: string[];
}

export interface Emoji {
  id: string;
  name: string;
  keywords: string[];
  skins: Skin[];
  version: number;
  emoticons?: string[];
}

export interface Skin {
  unified: string;
  native: string;
}

export interface Sheet {
  cols: number;
  rows: number;
}

interface EmojiSuggestionsProps {
  emojis: Emoji[];
  onSelect: (emoji: Emoji) => void;
  highlightedIndex: number;
}

export function EmojiSuggestions({
  emojis,
  onSelect,
  highlightedIndex,
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
          overflow: "auto",
        },
      ]}
    >
      {emojis.map((emoji, index) => (
        <Pressable
          key={emoji.id}
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
          <Text style={{ fontSize: 16, marginRight: 8 }}>
            {emoji.skins[0]?.native}
          </Text>
          <Text style={{ color: "white", fontSize: 14 }}>
            <Code style={[bg.gray[950]]}>:{emoji.id}:</Code> {emoji.name}
          </Text>
        </Pressable>
      ))}
    </View>
  );
}
