import { TouchableOpacity } from "react-native";
import { ScrollView, Text, View } from "tamagui";

export interface MentionSuggestion {
  did: string;
  handle: string;
  color?: {
    red: number;
    green: number;
    blue: number;
  };
}

interface MentionSuggestionsProps {
  suggestions: MentionSuggestion[];
  onSelect: (suggestion: MentionSuggestion) => void;
  highlightedIndex: number;
  setHighlightedIndex: (i: number) => void;
}

export default function MentionSuggestions({
  suggestions,
  onSelect,
  highlightedIndex,
  setHighlightedIndex,
}: MentionSuggestionsProps) {
  if (suggestions.length === 0) return null;

  const getRgbColor = (color?: MentionSuggestion["color"]) =>
    color ? `rgb(${color.red}, ${color.green}, ${color.blue})` : "$accentColor";

  return (
    <View
      position="absolute"
      left={0}
      right={0}
      bottom="100%"
      marginBottom={44}
      backgroundColor="$background"
      borderRadius={4}
      maxHeight={200}
      minWidth={200}
      zIndex={100000}
      shadowColor="$shadowColor"
      shadowOffset={{ width: 0, height: 2 }}
      shadowOpacity={0.25}
      shadowRadius={4}
      style={{
        pointerEvents: "auto",
      }}
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
      <ScrollView>
        {suggestions.map((suggestion, i) => (
          <View
            key={suggestion.did}
            onMouseEnter={() => setHighlightedIndex(i)}
          >
            <TouchableOpacity onPress={() => onSelect(suggestion)} style={{}}>
              <View
                padding="$2"
                backgroundColor={
                  i === highlightedIndex ? "$accentBackground" : "transparent"
                }
                style={{
                  borderBottomWidth: 1,
                  borderBottomColor: "$borderColor",
                }}
              >
                <Text fontSize={14} color={getRgbColor(suggestion.color)}>
                  @{suggestion.handle}
                </Text>
              </View>
            </TouchableOpacity>
          </View>
        ))}
      </ScrollView>
    </View>
  );
}
