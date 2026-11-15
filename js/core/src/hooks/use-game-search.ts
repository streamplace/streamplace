import { useEffect, useState } from "react";
import { games, place, type StreamplaceAgent } from "streamplace";
import { getDidFromAtUri, getGameCoverUrl } from "../lib/game";

export interface GameSearchResult {
  uri: string;
  name: string;
  coverUrl?: string;
  genres?: string[];
}

export function useGameSearch(
  agent: StreamplaceAgent | null,
  query: string,
  enabled: boolean,
) {
  const [results, setResults] = useState<GameSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setResults([]);
    setError(false);
    const input = query.trim();
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
        const response = await agent.client.call(place.stream.game.search, {
          q: input,
          limit: 8,
        });
        const found = response.results
          .filter((game) =>
            games.gamesgamesgamesgames.defs.gameSummaryView.isTypeOf(game),
          )
          .map((game) => ({
            uri: game.uri,
            name: game.name,
            coverUrl: getGameCoverUrl(game.media, getDidFromAtUri(game.uri)),
            genres: (game as typeof game & { genres?: string[] }).genres,
          }));
        if (!cancelled) setResults(found);
      } catch {
        if (!cancelled) setError(true);
      } finally {
        if (!cancelled) setSearching(false);
      }
    }, 300);

    return () => {
      cancelled = true;
      clearTimeout(timeout);
    };
  }, [agent, enabled, query]);

  return { results, searching, error };
}
