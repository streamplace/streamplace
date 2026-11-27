import { getDidFromAtUri } from "@/lib/game";
import { useStore } from "@/lib/store";
import { cn } from "@/lib/utils";
import { useGameSearch, type GameSearchResult } from "@streamplace/core";
import { Search, X } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { place, StreamplaceAgent } from "streamplace";

/**
 * Activity picker for livestreams:
 *  - "Game" mode: search for actual games via the streamplace xrpc,
 *    show cover art + name. Stores an `ActivityGame` value
 *    with `$type: "place.stream.defs#activityGame"`.
 *  - "Other Activity" mode: predefined labels (Just Chatting, Music, …).
 *    Stores an `ActivityLabel` value.
 */

const ACTIVITY_LABELS: Array<{ value: string; display: string }> = [
  { value: "just_chatting", display: "Just Chatting" },
  { value: "music", display: "Music" },
  { value: "art", display: "Art" },
  { value: "software_dev", display: "Software Dev" },
  { value: "miniatures", display: "Miniatures" },
  { value: "events", display: "Events" },
  { value: "makers_crafting", display: "Makers" },
  { value: "cooking", display: "Cooking" },
  { value: "fitness", display: "Fitness" },
  { value: "sports", display: "Sports" },
];

type Mode = "game" | "label";

interface ActivityPickerProps {
  value: place.stream.livestream.Main["activity"] | undefined;
  onChange: (
    activity: place.stream.livestream.Main["activity"] | undefined,
  ) => void;
}

export function ActivityPicker({ value, onChange }: ActivityPickerProps) {
  const { t } = useTranslation("common");
  const nodeUrl = useStore((state) => state.url);
  const agent = useMemo(
    () => (nodeUrl ? new StreamplaceAgent(nodeUrl) : null),
    [nodeUrl],
  );

  // Determine initial mode from the current value
  const initialMode = useMemo<Mode>(() => {
    if (value?.$type === "place.stream.defs#activityLabel") return "label";
    if (value?.$type === "place.stream.defs#activityGame") return "game";
    return "game";
  }, []);

  const [mode, setMode] = useState<Mode>(initialMode);
  const [query, setQuery] = useState("");

  const selectedGame =
    value?.$type === "place.stream.defs#activityGame"
      ? (value as { uri: string; name: string })
      : null;
  const selectedLabel =
    value?.$type === "place.stream.defs#activityLabel"
      ? (value as { label: string })
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

  const selectGame = (game: GameSearchResult) => {
    onChange({
      $type: "place.stream.defs#activityGame",
      uri: game.uri,
      name: game.name,
    } as unknown as place.stream.livestream.Main["activity"]);
    setQuery("");
  };

  const clearActivity = () => {
    onChange(undefined);
    setQuery("");
  };

  return (
    <div className="space-y-1.5">
      <label className="mb-1 block text-sm text-(--color-fg-muted)">
        {t("activity", { defaultValue: "Activity" })}
      </label>

      {/* Mode toggle */}
      <div className="mb-2 flex gap-1">
        <ModeButton
          active={mode === "game"}
          onClick={() => {
            setMode("game");
            if (selectedLabel) clearActivity();
          }}
          label={t("activity-mode-game", { defaultValue: "Game" })}
        />
        <ModeButton
          active={mode === "label"}
          onClick={() => {
            setMode("label");
            if (selectedGame) clearActivity();
          }}
          label={t("activity-mode-other", { defaultValue: "Other Activity" })}
        />
      </div>

      {/* Game mode */}
      {mode === "game" &&
        (selectedGame ? (
          <SelectedGameCard
            uri={selectedGame.uri}
            name={selectedGame.name}
            onClear={clearActivity}
          />
        ) : (
          <div>
            <div className="relative">
              <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-(--color-fg-muted)" />
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("activity-search-placeholder", {
                  defaultValue: "Search for a game…",
                })}
                className="focus:ring-ring h-8 w-full rounded-md border border-(--color-border) bg-(--color-bg) pr-2 pl-8 text-sm text-(--color-fg) placeholder:text-(--color-fg-muted) focus:ring-1 focus:outline-none"
              />
            </div>
            {searching && (
              <div className="px-2 py-1.5 text-xs text-(--color-fg-muted)">
                {t("searching", { defaultValue: "Searching…" })}
              </div>
            )}
            {!searching && (results.length > 0 || hasMore) && (
              <div
                data-testid="game-results"
                onScroll={(event) => {
                  const list = event.currentTarget;
                  if (
                    !loadMoreError &&
                    list.scrollHeight - list.scrollTop - list.clientHeight <= 64
                  )
                    void loadMore();
                }}
                className="mt-1 max-h-64 overflow-y-auto rounded-md border border-(--color-border) bg-(--color-bg)"
              >
                {results.map((game) => (
                  <button
                    key={game.uri}
                    type="button"
                    onClick={() => selectGame(game)}
                    className="flex w-full items-center gap-2 px-2 py-1.5 text-left transition-colors hover:bg-(--color-bg-elevated)"
                  >
                    {game.coverUrl ? (
                      <img
                        src={game.coverUrl}
                        alt=""
                        className="h-10 w-8 shrink-0 rounded-sm bg-(--color-bg-elevated) object-cover"
                      />
                    ) : (
                      <div className="h-10 w-8 shrink-0 rounded-sm bg-(--color-bg-elevated)" />
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="line-clamp-2 text-sm font-medium">
                        {game.name}
                      </div>
                    </div>
                  </button>
                ))}
                {hasMore && (
                  <button
                    type="button"
                    onClick={() => void loadMore()}
                    disabled={loadingMore}
                    className="w-full px-2 py-2 text-center text-xs text-(--color-fg-muted) hover:bg-(--color-bg-elevated) disabled:opacity-40"
                  >
                    {loadingMore
                      ? t("loading", { defaultValue: "Loading..." })
                      : loadMoreError
                        ? t("try-again", { defaultValue: "Try again" })
                        : t("load-more", { defaultValue: "Load more" })}
                  </button>
                )}
              </div>
            )}
            {!searching && query.trim() && results.length === 0 && !hasMore && (
              <div className="px-2 py-1.5 text-xs text-(--color-fg-muted)">
                {searchError
                  ? t("search-failed", { defaultValue: "Search failed" })
                  : t("no-results", { defaultValue: "No games found" })}
              </div>
            )}
          </div>
        ))}

      {/* Label mode */}
      {mode === "label" && (
        <div className="flex flex-wrap gap-1.5">
          {ACTIVITY_LABELS.map(({ value: labelValue, display }) => {
            const isSelected = selectedLabel?.label === labelValue;
            return (
              <button
                key={labelValue}
                type="button"
                onClick={() =>
                  onChange(
                    isSelected
                      ? undefined
                      : ({
                          $type: "place.stream.defs#activityLabel",
                          label: labelValue,
                        } as unknown as place.stream.livestream.Main["activity"]),
                  )
                }
                className={cn(
                  "rounded-full border px-2.5 py-1 text-sm transition-colors",
                  isSelected
                    ? "border-(--color-accent) bg-(--color-accent) text-(--color-accent-fg)"
                    : "border-(--color-border) text-(--color-fg-muted) hover:border-(--color-border-strong)",
                )}
              >
                {display}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function ModeButton({
  active,
  onClick,
  label,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-md border px-2.5 py-0.5 text-xs transition-colors",
        active
          ? "border-(--color-accent) bg-(--color-accent)/15 text-(--color-accent)"
          : "border-(--color-border) text-(--color-fg-muted) hover:border-(--color-border-strong)",
      )}
    >
      {label}
    </button>
  );
}

function SelectedGameCard({
  uri,
  name,
  onClear,
}: {
  uri: string;
  name: string;
  onClear: () => void;
}) {
  const did = getDidFromAtUri(uri);
  // We don't have the cover URL here without re-fetching; show the name
  // and let the X clear it. (Re-fetching is what the RN does in
  // useEffect, but it'd be extra round-trips; the search results above
  // already showed the cover so the user has seen it.)
  void did;
  return (
    <div className="flex items-center gap-2 rounded-md border border-(--color-accent)/40 bg-(--color-accent)/5 p-2">
      <div className="min-w-0 flex-1">
        <div className="line-clamp-2 text-sm font-medium text-(--color-accent)">
          {name}
        </div>
        <div className="truncate text-[10px] text-(--color-fg-muted)">
          {uri}
        </div>
      </div>
      <button
        type="button"
        onClick={onClear}
        className="shrink-0 rounded p-1 text-(--color-fg-muted) hover:bg-(--color-bg-elevated) hover:text-(--color-fg)"
        aria-label="Clear activity"
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
}
