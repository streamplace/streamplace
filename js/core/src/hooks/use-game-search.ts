import { useEffect, useRef, useState } from "react";
import { games, place, type StreamplaceAgent } from "streamplace";
import { getDidFromAtUri, getGameCoverUrl } from "../lib/game";

export interface GameSearchResult {
  uri: string;
  name: string;
  coverUrl?: string;
  genres?: string[];
}

function gameQuery(
  input: string,
):
  | { uri: NonNullable<place.stream.game.getGame.$Params["uri"]> }
  | { slug: string }
  | { q: string } {
  if (input.startsWith("at://"))
    return {
      uri: input as NonNullable<place.stream.game.getGame.$Params["uri"]>,
    };
  let url: URL;
  try {
    url = new URL(input);
  } catch {
    return { q: input };
  }
  const host = url.hostname.toLowerCase();
  if (
    !["http:", "https:"].includes(url.protocol) ||
    ![
      "cartridge.dev",
      "www.cartridge.dev",
      "igdb.com",
      "www.igdb.com",
    ].includes(host)
  ) {
    return { q: input };
  }
  const parts = decodeURIComponent(url.pathname)
    .replace(/^\/+|\/+$/g, "")
    .split("/");
  const cartridge = host === "cartridge.dev" || host === "www.cartridge.dev";
  if (
    url.username ||
    url.password ||
    parts.length !== 2 ||
    parts[0] !== (cartridge ? "game" : "games") ||
    !parts[1]
  ) {
    throw new Error("Invalid game link");
  }
  if (cartridge) return { slug: parts[1] };
  return { q: parts[1].replace(/--\d+$/, "").replace(/-/g, " ") };
}

export function useGameSearch(
  agent: StreamplaceAgent | null,
  query: string,
  enabled: boolean,
  onSelect: (game: GameSearchResult) => void,
) {
  const [results, setResults] = useState<GameSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState(false);
  const [resultsQuery, setResultsQuery] = useState("");
  const [submission, setSubmission] = useState(0);
  const pendingSelection = useRef<string | null>(null);
  const onSelectRef = useRef(onSelect);

  useEffect(() => {
    onSelectRef.current = onSelect;
  }, [onSelect]);

  const submit = (value = query) => {
    const input = value.trim();
    if (!input) return;
    if (input === resultsQuery && !searching && results.length > 0) {
      if (results.length === 1) onSelectRef.current(results[0]);
      return;
    }
    pendingSelection.current = input;
    setSubmission((count) => count + 1);
  };

  const onQueryChange = (next: string) => {
    if (next.trim() !== query.trim()) pendingSelection.current = null;
    if (!/^(https?:\/\/|at:\/\/)/i.test(next.trim())) return;
    let start = 0;
    while (
      start < query.length &&
      start < next.length &&
      query[start] === next[start]
    )
      start++;
    let oldEnd = query.length;
    let newEnd = next.length;
    while (
      oldEnd > start &&
      newEnd > start &&
      query[oldEnd - 1] === next[newEnd - 1]
    ) {
      oldEnd--;
      newEnd--;
    }
    if (newEnd - start > 1) submit(next);
  };

  useEffect(() => {
    let cancelled = false;
    setResults([]);
    setResultsQuery("");
    setError(false);
    const input = query.trim();
    if (pendingSelection.current !== input) pendingSelection.current = null;
    if (!enabled || !input) {
      pendingSelection.current = null;
      setSearching(false);
      return;
    }
    if (!agent) {
      setSearching(false);
      setError(true);
      return;
    }
    setSearching(true);
    const run = async () => {
      try {
        const lookup = gameQuery(input);
        let found: GameSearchResult[];
        if ("q" in lookup) {
          const response = await agent.client.call(place.stream.game.search, {
            q: lookup.q,
            limit: 8,
          });
          found = response.results
            .filter((game) =>
              games.gamesgamesgamesgames.defs.gameSummaryView.isTypeOf(game),
            )
            .map((game) => ({
              uri: game.uri,
              name: game.name,
              coverUrl: getGameCoverUrl(game.media, getDidFromAtUri(game.uri)),
              genres: (game as typeof game & { genres?: string[] }).genres,
            }));
        } else {
          try {
            const game = await agent.client.call(
              place.stream.game.getGame,
              lookup,
            );
            found = [
              {
                uri: game.uri,
                name: game.name,
                coverUrl: game.coverUrl,
                genres: game.genres,
              },
            ];
          } catch (err) {
            if (
              typeof err === "object" &&
              err !== null &&
              "status" in err &&
              err.status === 404
            ) {
              found = [];
            } else {
              throw err;
            }
          }
        }
        if (!cancelled) {
          const selectSingle = pendingSelection.current === input;
          pendingSelection.current = null;
          if (selectSingle && found.length === 1) {
            onSelectRef.current(found[0]);
          } else {
            setResults(found);
            setResultsQuery(input);
          }
        }
      } catch {
        if (!cancelled) {
          pendingSelection.current = null;
          setError(true);
        }
      } finally {
        if (!cancelled) setSearching(false);
      }
    };
    const timeout = setTimeout(
      run,
      pendingSelection.current === input ? 0 : 300,
    );

    return () => {
      cancelled = true;
      clearTimeout(timeout);
    };
  }, [agent, enabled, query, submission]);

  return { results, searching, error, submit, onQueryChange };
}
