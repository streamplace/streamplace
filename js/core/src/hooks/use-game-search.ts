import { useEffect, useRef, useState } from "react";
import { games, place, type StreamplaceAgent } from "streamplace";
import { getDidFromAtUri, getGameCoverUrl } from "../lib/game";

export interface GameSearchResult {
  uri: string;
  name: string;
  coverUrl?: string;
  genres?: string[];
}

async function searchGames(
  agent: StreamplaceAgent,
  q: string,
  cursor?: string,
) {
  const response = await agent.client.call(place.stream.game.search, {
    q,
    limit: 15,
    types: ["game"],
    ...(cursor ? { cursor } : {}),
  });
  return {
    results: response.results
      .filter((game) =>
        games.gamesgamesgamesgames.defs.gameSummaryView.isTypeOf(game),
      )
      .map((game) => ({
        uri: game.uri,
        name: game.name,
        coverUrl: getGameCoverUrl(game.media, getDidFromAtUri(game.uri)),
        genres: (game as typeof game & { genres?: string[] }).genres,
      })),
    cursor: response.cursor,
  };
}

export function useGameSearch(
  agent: StreamplaceAgent | null,
  query: string,
  enabled: boolean,
) {
  const input = query.trim();
  const [results, setResults] = useState<GameSearchResult[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [searching, setSearching] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState(false);
  const [loadMoreError, setLoadMoreError] = useState(false);
  const generation = useRef(0);
  const loadingMoreRef = useRef(false);

  useEffect(() => {
    const current = ++generation.current;
    loadingMoreRef.current = false;
    setResults([]);
    setNextCursor(undefined);
    setError(false);
    setLoadMoreError(false);
    setLoadingMore(false);
    if (!enabled || !input) {
      setSearching(false);
      return;
    }
    if (!agent) {
      setSearching(false);
      setError(true);
      return;
    }
    setSearching(true);
    const timeout = setTimeout(async () => {
      try {
        const response = await searchGames(agent, input);
        if (generation.current === current) {
          setResults(response.results);
          setNextCursor(response.cursor);
        }
      } catch {
        if (generation.current === current) setError(true);
      } finally {
        if (generation.current === current) setSearching(false);
      }
    }, 300);
    return () => {
      generation.current++;
      clearTimeout(timeout);
    };
  }, [agent, input, enabled]);

  const loadMore = async () => {
    if (
      !agent ||
      !enabled ||
      !nextCursor ||
      searching ||
      loadingMoreRef.current
    )
      return;
    const current = generation.current;
    const cursor = nextCursor;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    setLoadMoreError(false);
    try {
      const response = await searchGames(agent, input, cursor);
      if (generation.current === current) {
        setResults((existing) => {
          const seen = new Set(existing.map((game) => game.uri));
          return [
            ...existing,
            ...response.results.filter((game) => !seen.has(game.uri)),
          ];
        });
        setNextCursor(response.cursor === cursor ? undefined : response.cursor);
      }
    } catch {
      if (generation.current === current) setLoadMoreError(true);
    } finally {
      if (generation.current === current) {
        loadingMoreRef.current = false;
        setLoadingMore(false);
      }
    }
  };

  return {
    results,
    searching,
    error,
    hasMore: !!nextCursor,
    loadingMore,
    loadMoreError,
    loadMore,
  };
}
