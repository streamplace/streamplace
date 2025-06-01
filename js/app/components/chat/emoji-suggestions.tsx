import { TouchableOpacity } from "react-native";
import { ScrollView, Text, View } from "tamagui";

export interface EmojiSuggestion {
  emoji: string;
  shortcode: string;
  name: string;
}

interface EmojiSuggestionsProps {
  suggestions: EmojiSuggestion[];
  onSelect: (suggestion: EmojiSuggestion) => void;
  highlightedIndex: number;
  setHighlightedIndex: (i: number) => void;
}

export default function EmojiSuggestions({
  suggestions,
  onSelect,
  highlightedIndex,
  setHighlightedIndex,
}: EmojiSuggestionsProps) {
  if (suggestions.length === 0) return null;

  return (
    <View
      position="absolute"
      left={0}
      right={0}
      bottom="100%"
      marginBottom={44}
      backgroundColor="$background"
      borderRadius={4}
      zIndex={100000}
      shadowColor="$shadowColor"
      shadowOffset={{ width: 0, height: 2 }}
      shadowOpacity={0.25}
      shadowRadius={4}
      style={{ pointerEvents: "auto" }}
    >
      <Text
        fontSize={12}
        color="$color"
        padding="$2"
        opacity={0.7}
        style={{ borderBottomWidth: 1, borderBottomColor: "$borderColor" }}
      >
        ↑/↓ to navigate, Tab/Enter to select, Esc to close
      </Text>
      <ScrollView style={{ maxHeight: 240 }}>
        {suggestions.slice(0, 5).map((suggestion, i) => (
          <View
            key={suggestion.shortcode}
            onMouseEnter={() => setHighlightedIndex(i)}
          >
            <TouchableOpacity onPress={() => onSelect(suggestion)}>
              <View
                padding="$2"
                backgroundColor={
                  i === highlightedIndex ? "$accentBackground" : "transparent"
                }
                style={{
                  borderBottomWidth: 1,
                  borderBottomColor: "$borderColor",
                  flexDirection: "row",
                  alignItems: "center",
                  gap: 8,
                }}
              >
                <Text fontSize={18} style={{ width: 28 }}>
                  {suggestion.emoji}
                </Text>
                <Text
                  fontSize={14}
                  color="$accentColor"
                  style={{ minWidth: 70 }}
                >
                  {suggestion.shortcode}
                </Text>
                <Text fontSize={13} color="$color" opacity={0.7}>
                  {suggestion.name}
                </Text>
              </View>
            </TouchableOpacity>
          </View>
        ))}
      </ScrollView>
    </View>
  );
}
