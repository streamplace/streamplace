import { useEffect, useRef } from "react";
import { Platform, Pressable, ScrollView } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { Text, View } from "../..";
import { bg, layout, left, right } from "../../lib/theme/atoms";

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
        {
          bottom: "100%",
          borderRadius: 8,
          boxShadow: "0px 4px 6px rgba(0, 0, 0, 0.1)",
          maxHeight: 200,
          zIndex: 999999,
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
