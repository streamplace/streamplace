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
import { useGameSearch, type GameSearchResult } from "@streamplace/core";
import { Image } from "expo-image";
import { X } from "lucide-react-native";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import {
  Platform,
  Pressable,
  ScrollView,
  View,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
  type ViewInstance,
} from "react-native";
import { place, StreamplaceAgent } from "streamplace";

const { p, px, r, layout, borders, gap, flex } = zero;

interface ActivityPickerProps {
  value: place.stream.livestream.Main["activity"] | undefined;
  onChange: (
    activity: place.stream.livestream.Main["activity"] | undefined,
  ) => void;
}

interface DropdownPos {
  top?: number;
  bottom?: number;
  left: number;
  width: number;
  maxResultsHeight: number;
}

export default function ActivityPicker({
  value,
  onChange,
}: ActivityPickerProps) {
  const nodeUrl = useStreamplaceStore((s) => s.url);
  const agent = useMemo(
    () => (nodeUrl ? new StreamplaceAgent(nodeUrl) : null),
    [nodeUrl],
  );
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
  const [selectedCoverUrl, setSelectedCoverUrl] = useState<
    string | undefined
  >();
  const [selectedGenres, setSelectedGenres] = useState<string[]>([]);
  const [dropdownPos, setDropdownPos] = useState<DropdownPos | null>(null);
  const [labelsExpanded, setLabelsExpanded] = useState(false);
  const inputContainerRef = useRef<ViewInstance>(null);

  const selectedGame =
    value?.$type === "place.stream.defs#activityGame"
      ? (value as place.stream.defs.ActivityGame)
      : null;
  const selectedLabel =
    value?.$type === "place.stream.defs#activityLabel"
      ? (value as place.stream.defs.ActivityLabel)
      : null;

  const {
    results,
    searching,
    error: searchError,
    hasMore,
    loadingMore,
    loadMoreError,
    loadMore,
  } = useGameSearch(agent, query, mode === "game" && !selectedGame);

  const showResults = query.trim().length > 0;

  // The Game ⇄ Other Activity toggle only earns its place when both modes exist.
  // With games disabled it's a single orphaned control, so we drop it and show
  // the label chips directly.
  const availableModes = (["game", "label"] as const).filter(
    (m) => m !== "game" || gamesEnabled,
  );

  useEffect(() => {
    if (!selectedGame || !agent || selectedCoverUrl !== undefined) return;
    agent.client
      .call(place.stream.game.getGame, { uri: selectedGame.uri })
      .then((res) => {
        setSelectedCoverUrl(res.coverUrl);
        setSelectedGenres(res.genres ?? []);
      })
      .catch(() => {});
  }, [agent, selectedGame?.uri, selectedCoverUrl]);

  useLayoutEffect(() => {
    if (Platform.OS !== "web" || !inputContainerRef.current || !showResults) {
      setDropdownPos(null);
      return;
    }
    const el = inputContainerRef.current as unknown as HTMLElement;
    const rect = el.getBoundingClientRect();
    const below = window.innerHeight - rect.bottom;
    const controlsHeight = theme.touchTargets.minimum + theme.spacing[4];
    const above =
      below < theme.spacing[64] + controlsHeight && rect.top > below;
    const available = above ? rect.top : below;
    setDropdownPos({
      top: above ? undefined : rect.bottom + theme.spacing[1],
      bottom: above
        ? window.innerHeight - rect.top + theme.spacing[1]
        : undefined,
      left: rect.left,
      width: rect.width,
      maxResultsHeight: Math.min(
        theme.spacing[64],
        Math.max(
          theme.spacing[16],
          available - controlsHeight - theme.spacing[1],
        ),
      ),
    });
  }, [showResults, theme]);

  const selectGame = (game: GameSearchResult) => {
    onChange({
      $type: "place.stream.defs#activityGame",
      uri: game.uri as any,
      name: game.name,
    });
    setSelectedCoverUrl(game.coverUrl);
    setQuery("");
  };

  const clearActivity = () => {
    onChange(undefined);
    setSelectedCoverUrl(undefined);
    setSelectedGenres([]);
    setQuery("");
  };

  const onResultsScroll = (event: NativeSyntheticEvent<NativeScrollEvent>) => {
    const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
    if (
      !loadMoreError &&
      contentSize.height - contentOffset.y - layoutMeasurement.height <=
        theme.spacing[16]
    ) {
      void loadMore();
    }
  };

  const loadMoreControl = hasMore && (
    <Pressable
      accessibilityRole="button"
      disabled={loadingMore}
      onPress={() => void loadMore()}
      style={[
        p[2],
        {
          minHeight: theme.touchTargets.minimum,
          opacity: loadingMore ? 0.4 : 1,
        },
      ]}
    >
      <Text size="sm" color="muted">
        {loadingMore ? "Loading..." : loadMoreError ? "Try again" : "Load more"}
      </Text>
    </Pressable>
  );

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
      {!searching && results.length === 0 && !hasMore && (
        <Text size="sm" color="muted" style={[p[2]]}>
          {searchError ? "Search failed" : "No games found"}
        </Text>
      )}
      <ScrollView
        style={{
          maxHeight: dropdownPos?.maxResultsHeight ?? theme.spacing[64],
        }}
        onScroll={onResultsScroll}
        scrollEventThrottle={16}
      >
        {results.map((game) => (
          <Pressable
            key={game.uri}
            onPress={() => selectGame(game)}
            style={[
              layout.flex.row,
              layout.flex.alignCenter,
              p[2],
              { gap: 10 },
            ]}
          >
            <Image
              source={{ uri: game.coverUrl }}
              style={{ width: 32, height: 43, borderRadius: 3 }}
              contentFit="cover"
            />
            <View style={[flex.values[1]]}>
              <Text numberOfLines={2}>{game.name}</Text>
            </View>
          </Pressable>
        ))}
        {loadMoreControl}
      </ScrollView>
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
                <Text numberOfLines={2} style={{ color: c.primary }}>
                  {selectedGame.name}
                </Text>
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
                      bottom: dropdownPos.bottom,
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
                onContentScroll={onResultsScroll}
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
                  {!searching &&
                    query.trim().length > 0 &&
                    results.length === 0 &&
                    !hasMore && (
                      <Text size="sm" color="muted">
                        {searchError ? "Search failed" : "No games found"}
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
                      <Text numberOfLines={2} style={[flex.values[1]]}>
                        {game.name}
                      </Text>
                    </View>
                  </DropdownMenuItem>
                ))}
                {loadMoreControl}
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
            selectedLabel && !base.some((l) => l.value === selectedLabel.label)
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
                  style={[px[3], { paddingVertical: 6, borderRadius: 16 }]}
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
