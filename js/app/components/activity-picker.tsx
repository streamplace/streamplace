import {
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Input,
  ResponsiveDropdownMenuContent,
  Text,
  useTheme,
  zero,
} from "@streamplace/components";
import { ACTIVITY_LABELS } from "@streamplace/components/src/lib/metadata-constants";
import { useStreamplaceStore } from "@streamplace/components/src/streamplace-store/streamplace-store";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { Image } from "expo-image";
import { X } from "lucide-react-native";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Platform, Pressable, View } from "react-native";
import {
  GamesGamesgamesgamesgamesDefs,
  PlaceStreamDefs,
  PlaceStreamLivestream,
} from "streamplace";
import { getDidFromAtUri, getGameCoverUrl } from "../utils/game";

const { p, px, r, layout, borders, gap, flex } = zero;

interface GameResult {
  uri: string;
  name: string;
  coverUrl?: string;
  genres?: string[];
}

interface ActivityPickerProps {
  value: PlaceStreamLivestream.Record["activity"] | undefined;
  onChange: (
    activity: PlaceStreamLivestream.Record["activity"] | undefined,
  ) => void;
}

interface DropdownPos {
  top: number;
  left: number;
  width: number;
}

export default function ActivityPicker({
  value,
  onChange,
}: ActivityPickerProps) {
  const agent = usePDSAgent();
  const { theme, zero: z } = useTheme();
  const c = theme.colors;
  const gamesEnabled = useStreamplaceStore((s) => s.gamesEnabled);

  const [mode, setMode] = useState<"game" | "label">(() => {
    if (value?.$type === "place.stream.defs#activityLabel") return "label";
    if (value?.$type === "place.stream.defs#activityGame" && gamesEnabled)
      return "game";
    return gamesEnabled ? "game" : "label";
  });
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<GameResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedCoverUrl, setSelectedCoverUrl] = useState<
    string | undefined
  >();
  const [selectedGenres, setSelectedGenres] = useState<string[]>([]);
  const [dropdownPos, setDropdownPos] = useState<DropdownPos | null>(null);
  const [labelsExpanded, setLabelsExpanded] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inputContainerRef = useRef<View>(null);

  const selectedGame =
    value?.$type === "place.stream.defs#activityGame"
      ? (value as PlaceStreamDefs.ActivityGame)
      : null;
  const selectedLabel =
    value?.$type === "place.stream.defs#activityLabel"
      ? (value as PlaceStreamDefs.ActivityLabel)
      : null;

  const showResults = searching || results.length > 0;

  // The Game ⇄ Other Activity toggle only earns its place when both modes exist.
  // With games disabled it's a single orphaned control, so we drop it and show
  // the label chips directly.
  const availableModes = (["game", "label"] as const).filter(
    (m) => m !== "game" || gamesEnabled,
  );

  useEffect(() => {
    if (!selectedGame || !agent || selectedCoverUrl !== undefined) return;
    agent.place.stream.game
      .getGame({ uri: selectedGame.uri })
      .then((res) => {
        setSelectedCoverUrl(res.data.coverUrl);
        setSelectedGenres(res.data.genres ?? []);
      })
      .catch(() => {});
  }, [selectedGame?.uri]);

  useLayoutEffect(() => {
    if (Platform.OS !== "web" || !inputContainerRef.current || !showResults) {
      setDropdownPos(null);
      return;
    }
    const el = inputContainerRef.current as unknown as HTMLElement;
    const rect = el.getBoundingClientRect();
    setDropdownPos({
      top: rect.bottom + 4,
      left: rect.left,
      width: rect.width,
    });
  }, [showResults]);

  useEffect(() => {
    if (!query.trim() || !agent) {
      setResults([]);
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
          if (!GamesGamesgamesgamesgamesDefs.isGameSummaryView(result))
            continue;
          const did = getDidFromAtUri(result.uri);
          const cover = getGameCoverUrl(result.media, did);
          games.push({
            uri: result.uri,
            name: result.name,
            coverUrl: cover,
            genres: result.genres,
          });
        }
        setResults(games);
      } catch (e) {
        console.error("game search error:", e);
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
    setSelectedCoverUrl(game.coverUrl);
    setSelectedGenres(game.genres ?? []);
    setQuery("");
    setResults([]);
  };

  const clearActivity = () => {
    onChange(undefined);
    setSelectedCoverUrl(undefined);
    setSelectedGenres([]);
    setQuery("");
    setResults([]);
  };

  const resultsList = (
    <View
      style={[
        r.md,
        borders.width.thin,
        {
          borderColor: c.border,
          backgroundColor: c.popover,
          overflow: "hidden",
        },
      ]}
    >
      {searching && (
        <Text style={[p[2], { fontSize: 12, color: c.mutedForeground }]}>
          Searching...
        </Text>
      )}
      {results.map((game) => (
        <Pressable
          key={game.uri}
          onPress={() => selectGame(game)}
          style={[layout.flex.row, layout.flex.alignCenter, p[2], { gap: 10 }]}
        >
          <Image
            source={{ uri: game.coverUrl }}
            style={{ width: 32, height: 43, borderRadius: 3 }}
            contentFit="cover"
          />
          <View style={[flex.values[1]]}>
            <Text numberOfLines={1}>{game.name}</Text>
            {game.genres && game.genres.length > 0 && (
              <Text
                numberOfLines={1}
                style={{ fontSize: 11, color: c.mutedForeground }}
              >
                {game.genres.join(" · ")}
              </Text>
            )}
          </View>
        </Pressable>
      ))}
    </View>
  );

  return (
    <View style={[gap.all[2]]}>
      {availableModes.length > 1 && (
        <View style={[layout.flex.row, gap.all[2]]}>
          {availableModes.map((m) => (
            <Pressable
              key={m}
              onPress={() => {
                setMode(m);
                if (m === "game" && selectedLabel) onChange(undefined);
                if (m === "label" && selectedGame) onChange(undefined);
              }}
              style={[
                px[3],
                borders.width.thin,
                { paddingVertical: 6, borderRadius: 6 },
                { borderColor: mode === m ? c.border : c.border },
                {
                  backgroundColor: mode === m ? c.card : "transparent",
                },
              ]}
            >
              <Text
                style={{
                  color: mode === m ? c.primary : c.mutedForeground,
                  fontSize: 13,
                }}
              >
                {m === "game" ? "Game" : "Other Activity"}
              </Text>
            </Pressable>
          ))}
        </View>
      )}

      {mode === "game" && (
        <View>
          {selectedGame ? (
            <View
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                r.md,
                borders.width.thin,
                gap.all[2],
                p[2],
                { borderColor: c.border, backgroundColor: c.card },
              ]}
            >
              {selectedCoverUrl && (
                <Image
                  source={{ uri: selectedCoverUrl }}
                  style={{ width: 40, height: 53, borderRadius: 4 }}
                  contentFit="cover"
                />
              )}
              <View style={[flex.values[1]]}>
                <Text style={{ color: c.primary }}>{selectedGame.name}</Text>
                {selectedGenres.length > 0 && (
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.wrap.wrap,
                      { marginTop: 4, rowGap: 4, columnGap: 2 },
                    ]}
                  >
                    {selectedGenres.map((g) => (
                      <View
                        key={g}
                        style={[z.bg.muted, px[2], r.full, { marginRight: 4 }]}
                      >
                        <Text size="xs" leading="snug" color="muted">
                          {g}
                        </Text>
                      </View>
                    ))}
                  </View>
                )}
              </View>
              <Pressable onPress={clearActivity}>
                <X size={16} color={c.mutedForeground} />
              </Pressable>
            </View>
          ) : Platform.OS === "web" ? (
            <View ref={inputContainerRef}>
              <Input
                value={query}
                onChangeText={setQuery}
                placeholder="Search for a game..."
              />
              {showResults &&
                dropdownPos &&
                // kind of hacky, but using a portal to avoid z-index and overflow issues vs
                // rendering the dropdown absolutely within the container
                require("react-dom").createPortal(
                  <View
                    style={{
                      position: "fixed" as any,
                      top: dropdownPos.top,
                      left: dropdownPos.left,
                      width: dropdownPos.width,
                      zIndex: 9999,
                    }}
                  >
                    {resultsList}
                  </View>,
                  document.body,
                )}
            </View>
          ) : (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Pressable
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    r.md,
                    p[2],
                    borders.width.thin,
                    gap.all[2],
                    {
                      borderColor: c.border,
                      backgroundColor: c.input,
                    },
                  ]}
                >
                  <Text
                    style={[
                      flex.values[1],
                      { color: c.mutedForeground, fontSize: 14 },
                    ]}
                  >
                    Search for a game...
                  </Text>
                </Pressable>
              </DropdownMenuTrigger>
              <ResponsiveDropdownMenuContent
                align="start"
                style={{ minWidth: 300 }}
              >
                <View style={[p[2]]}>
                  <Input
                    value={query}
                    onChangeText={setQuery}
                    placeholder="Search for a game..."
                    autoFocus
                  />
                  {searching && (
                    <Text
                      style={{
                        fontSize: 12,
                        color: c.mutedForeground,
                        marginTop: 4,
                        paddingHorizontal: 4,
                      }}
                    >
                      Searching...
                    </Text>
                  )}
                </View>
                {results.map((game) => (
                  <DropdownMenuItem
                    key={game.uri}
                    onPress={() => selectGame(game)}
                  >
                    <View
                      style={[
                        layout.flex.row,
                        layout.flex.alignCenter,
                        { gap: 10 },
                      ]}
                    >
                      <Image
                        source={{ uri: game.coverUrl }}
                        style={{ width: 32, height: 43, borderRadius: 3 }}
                        contentFit="cover"
                      />
                      <Text numberOfLines={1} style={[flex.values[1]]}>
                        {game.name}
                      </Text>
                    </View>
                  </DropdownMenuItem>
                ))}
              </ResponsiveDropdownMenuContent>
            </DropdownMenu>
          )}
        </View>
      )}

      {mode === "label" &&
        (() => {
          // Collapse the category cloud so it doesn't dominate the panel: show
          // the first row's worth, always keep the selected chip visible, and
          // expand the rest behind a "+N more".
          const COLLAPSED_COUNT = 6;
          const base = labelsExpanded
            ? ACTIVITY_LABELS
            : ACTIVITY_LABELS.slice(0, COLLAPSED_COUNT);
          const selectedHidden =
            selectedLabel &&
            !base.some((l) => l.value === selectedLabel.label)
              ? ACTIVITY_LABELS.find((l) => l.value === selectedLabel.label)
              : undefined;
          const shown = selectedHidden ? [...base, selectedHidden] : base;
          const hiddenCount = ACTIVITY_LABELS.length - COLLAPSED_COUNT;

          return (
            <View
              style={[{ flexDirection: "row", flexWrap: "wrap" }, gap.all[2]]}
            >
              {shown.map(({ value: labelValue, display }) => {
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
                    style={[
                      px[3],
                      borders.width.thin,
                      { paddingVertical: 6, borderRadius: 16 },
                      { borderColor: selected ? c.primary : c.border },
                      { backgroundColor: selected ? c.primary : "transparent" },
                    ]}
                  >
                    <Text
                      style={{
                        color: selected ? c.primaryForeground : c.foreground,
                        fontSize: 13,
                      }}
                    >
                      {display}
                    </Text>
                  </Pressable>
                );
              })}
              {hiddenCount > 0 && (
                <Pressable
                  onPress={() => setLabelsExpanded((v) => !v)}
                  style={[
                    px[3],
                    { paddingVertical: 6, borderRadius: 16 },
                  ]}
                >
                  <Text style={{ color: c.mutedForeground, fontSize: 13 }}>
                    {labelsExpanded ? "Show less" : `+${hiddenCount} more`}
                  </Text>
                </Pressable>
              )}
            </View>
          );
        })()}
    </View>
  );
}
