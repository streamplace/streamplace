import { Text } from "@streamplace/components";
import { usePossiblyUnauthedPDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { useEffect, useRef, useState } from "react";
import { Pressable, ScrollView, TextInput, View } from "react-native";
import { PlaceStreamDefs, PlaceStreamLivestream } from "streamplace";

const ACTIVITY_LABELS: Array<{
  value: PlaceStreamDefs.ActivityLabel["label"];
  display: string;
}> = [
  { value: "just_chatting", display: "Just Chatting" },
  { value: "music", display: "Music" },
  { value: "art", display: "Art" },
  { value: "programming", display: "Programming" },
  { value: "cooking", display: "Cooking" },
  { value: "fitness", display: "Fitness" },
  { value: "sports", display: "Sports" },
];

interface GameResult {
  uri: string;
  name: string;
}

interface ActivityPickerProps {
  value: PlaceStreamLivestream.Record["activity"] | undefined;
  onChange: (
    activity: PlaceStreamLivestream.Record["activity"] | undefined,
  ) => void;
}

export default function ActivityPicker({
  value,
  onChange,
}: ActivityPickerProps) {
  const agent = usePossiblyUnauthedPDSAgent();
  const [mode, setMode] = useState<"game" | "label">(
    value?.$type === "place.stream.defs#activityLabel" ? "label" : "game",
  );
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<GameResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [showResults, setShowResults] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const selectedGame =
    value?.$type === "place.stream.defs#activityGame"
      ? (value as PlaceStreamDefs.ActivityGame)
      : null;
  const selectedLabel =
    value?.$type === "place.stream.defs#activityLabel"
      ? (value as PlaceStreamDefs.ActivityLabel)
      : null;

  useEffect(() => {
    if (!query.trim() || !agent) {
      setResults([]);
      setShowResults(false);
      return;
    }

    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      setSearching(true);
      try {
        const res = await agent.place.stream.game.search({
          q: query,
          limit: 8,
        });
        const games: GameResult[] = [];
        for (const result of res.data.results) {
          if (
            result.$type === "games.gamesgamesgamesgames.defs#gameSummaryView"
          ) {
            const view = result as { uri: string; name: string };
            if (view.uri && view.name) {
              games.push({ uri: view.uri, name: view.name });
            }
          }
        }
        setResults(games);
        setShowResults(true);
      } catch {
        setResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);

    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, agent]);

  const selectGame = (game: GameResult) => {
    onChange({
      $type: "place.stream.defs#activityGame",
      uri: game.uri,
      name: game.name,
    });
    setQuery("");
    setShowResults(false);
  };

  const clearActivity = () => {
    onChange(undefined);
    setQuery("");
  };

  return (
    <View style={{ gap: 8 }}>
      {/* Mode toggle */}
      <View style={{ flexDirection: "row", gap: 8 }}>
        <Pressable
          onPress={() => {
            setMode("game");
            if (selectedLabel) onChange(undefined);
          }}
          style={{
            paddingHorizontal: 12,
            paddingVertical: 6,
            borderRadius: 6,
            borderWidth: 1,
            borderColor: mode === "game" ? "#0066cc" : "#ccc",
            backgroundColor: mode === "game" ? "#e8f0fe" : "transparent",
          }}
        >
          <Text
            style={{
              color: mode === "game" ? "#0066cc" : "#666",
              fontSize: 13,
            }}
          >
            Game
          </Text>
        </Pressable>
        <Pressable
          onPress={() => {
            setMode("label");
            if (selectedGame) onChange(undefined);
          }}
          style={{
            paddingHorizontal: 12,
            paddingVertical: 6,
            borderRadius: 6,
            borderWidth: 1,
            borderColor: mode === "label" ? "#0066cc" : "#ccc",
            backgroundColor: mode === "label" ? "#e8f0fe" : "transparent",
          }}
        >
          <Text
            style={{
              color: mode === "label" ? "#0066cc" : "#666",
              fontSize: 13,
            }}
          >
            Other Activity
          </Text>
        </Pressable>
      </View>

      {mode === "game" && (
        <View style={{ position: "relative" }}>
          {selectedGame ? (
            <View
              style={{
                flexDirection: "row",
                alignItems: "center",
                gap: 8,
                padding: 10,
                borderWidth: 1,
                borderColor: "#0066cc",
                borderRadius: 8,
                backgroundColor: "#e8f0fe",
              }}
            >
              <Text style={{ flex: 1, color: "#0066cc" }}>
                {selectedGame.name}
              </Text>
              <Pressable onPress={clearActivity}>
                <Text style={{ color: "#666", fontSize: 18, lineHeight: 18 }}>
                  ×
                </Text>
              </Pressable>
            </View>
          ) : (
            <TextInput
              value={query}
              onChangeText={setQuery}
              placeholder="Search for a game..."
              style={{
                borderWidth: 1,
                borderColor: "#ccc",
                borderRadius: 8,
                padding: 10,
              }}
            />
          )}
          {showResults && results.length > 0 && (
            <View
              style={{
                position: "absolute",
                top: "100%",
                left: 0,
                right: 0,
                zIndex: 100,
                backgroundColor: "white",
                borderWidth: 1,
                borderColor: "#ccc",
                borderRadius: 8,
                marginTop: 4,
                maxHeight: 220,
                overflow: "hidden",
              }}
            >
              <ScrollView keyboardShouldPersistTaps="handled">
                {results.map((game) => (
                  <Pressable
                    key={game.uri}
                    onPress={() => selectGame(game)}
                    style={({ pressed }) => ({
                      padding: 10,
                      backgroundColor: pressed ? "#f0f0f0" : "white",
                      borderBottomWidth: 1,
                      borderBottomColor: "#eee",
                    })}
                  >
                    <Text numberOfLines={1}>{game.name}</Text>
                  </Pressable>
                ))}
              </ScrollView>
            </View>
          )}
          {searching && (
            <Text style={{ fontSize: 12, color: "#999", marginTop: 4 }}>
              Searching...
            </Text>
          )}
        </View>
      )}

      {mode === "label" && (
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
          {ACTIVITY_LABELS.map(({ value: labelValue, display }) => {
            const selected = selectedLabel?.label === labelValue;
            return (
              <Pressable
                key={labelValue}
                onPress={() =>
                  onChange(
                    selected
                      ? undefined
                      : {
                          $type: "place.stream.defs#activityLabel",
                          label: labelValue,
                        },
                  )
                }
                style={{
                  paddingHorizontal: 12,
                  paddingVertical: 6,
                  borderRadius: 16,
                  borderWidth: 1,
                  borderColor: selected ? "#0066cc" : "#ccc",
                  backgroundColor: selected ? "#0066cc" : "transparent",
                }}
              >
                <Text
                  style={{ color: selected ? "white" : "#333", fontSize: 13 }}
                >
                  {display}
                </Text>
              </Pressable>
            );
          })}
        </View>
      )}
    </View>
  );
}
