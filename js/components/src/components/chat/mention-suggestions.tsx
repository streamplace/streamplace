import { Pressable, ScrollView } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { Text, View } from "../..";
import { bg, layout, left, right, zIndex } from "../../lib/theme/atoms";

interface MentionSuggestionsProps {
  authors: Map<string, ChatMessageViewHydrated["chatProfile"]>;
  onSelect: (authorHandle: string) => void;
  highlightedIndex: number;
}

export function MentionSuggestions({
  authors,
  onSelect,
  highlightedIndex,
}: MentionSuggestionsProps) {
  if (!authors || authors.size === 0) {
    return null; // No authors to display
  }

  const authorHandles = Array.from(authors.keys());
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
          marginBottom: 8,
          borderRadius: 8,
          boxShadow: "0px 4px 6px rgba(0, 0, 0, 0.1)",
        },
      ]}
    >
      <ScrollView>
        {authorHandles.map((handle, index) => {
          let profile = authors.get(handle);
          return (
            <Pressable
              key={handle}
              onPress={() => onSelect(handle)}
              style={[
                {
                  padding: 8,
                  flexDirection: "row",
                  alignItems: "center",
                  backgroundColor:
                    index === highlightedIndex
                      ? "rgba(0, 0, 0, 0.1)"
                      : "rgba(0, 0, 0, 0.5)",
                },
              ]}
            >
              <Text
                style={{
                  color: profile?.color
                    ? `rgb(${profile.color.red}, ${profile.color.green}, ${profile.color.blue})`
                    : "black",
                  fontWeight: "bold",
                }}
              >
                @{handle}
              </Text>
            </Pressable>
          );
        })}
      </ScrollView>
    </View>
  );
}
