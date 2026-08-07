import { place } from "streamplace";
// Search page. Searches actors via searchActorsTypeahead and shows
// results as profile cards linking to their streams.
import useAvatars from "@/hooks/use-avatars";
import { usePDSAgent } from "@/lib/store/hooks";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Search } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

interface SearchResult {
  did: string;
  handle: string;
  displayName?: string;
}

export const Route = createFileRoute("/search")({
  validateSearch: (
    search: Record<string, unknown>,
  ): { q: string | undefined } => ({
    q: typeof search.q === "string" ? search.q : undefined,
  }),
  component: SearchPage,
});

function SearchPage() {
  const { t } = useTranslation("common");
  const { q: initialQ } = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });
  const agent = usePDSAgent();
  const [query, setQuery] = useState(initialQ ?? "");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const dids = results.map((r) => r.did);
  const avatars = useAvatars(dids);

  const searchActors = useCallback(
    async (q: string) => {
      if (!agent || !q.trim()) {
        setResults([]);
        return;
      }
      try {
        setSearching(true);
        const response = await agent.client.call(
          place.stream.live.searchActorsTypeahead,
          {
            q,
            limit: 20,
          },
        );
        setResults(
          response.actors.map((a: any) => ({
            did: a.did,
            handle: a.handle,
            displayName: a.displayName,
          })),
        );
      } catch (error) {
        console.error("Search failed:", error);
        setResults([]);
      } finally {
        setSearching(false);
      }
    },
    [agent],
  );

  // Sync initial query param into the input and trigger a search.
  useEffect(() => {
    if (initialQ) {
      searchActors(initialQ);
    }
    inputRef.current?.focus();
  }, [initialQ, searchActors]);

  const handleChange = (value: string) => {
    setQuery(value);
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    if (value.trim()) {
      timeoutRef.current = setTimeout(() => {
        searchActors(value);
        navigate({ search: { q: value }, replace: true });
      }, 300);
    } else {
      setResults([]);
      navigate({ search: { q: undefined }, replace: true });
    }
  };

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-6">
      <div className="relative mb-6">
        <Search className="absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-(--color-fg-muted)" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => handleChange(e.target.value)}
          placeholder={t("search-for-streamers")}
          className="focus:ring-ring h-10 w-full rounded-md border border-(--color-border) bg-(--color-bg) pr-4 pl-9 text-(--color-fg) placeholder:text-(--color-fg-muted) focus:ring-1 focus:outline-none"
        />
      </div>

      {searching && results.length === 0 && (
        <div className="flex justify-center py-12">
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-(--color-border) border-t-(--color-accent)" />
        </div>
      )}

      {!searching && query.trim() && results.length === 0 && (
        <div className="py-12 text-center">
          <p className="text-(--color-fg-muted)">
            {t("no-results-for", { query })}
          </p>
        </div>
      )}

      {results.length > 0 && (
        <div className="space-y-1">
          {results.map((actor) => (
            <Link
              key={actor.did}
              to="/$user"
              params={{ user: actor.handle }}
              className="flex items-center gap-3 rounded-lg p-3 transition-colors hover:bg-(--color-bg-elevated)"
            >
              {avatars[actor.did]?.avatar ? (
                <img
                  src={avatars[actor.did].avatar}
                  alt=""
                  className="h-10 w-10 rounded-full bg-(--color-bg) object-cover"
                />
              ) : (
                <div className="flex h-10 w-10 items-center justify-center rounded-full border border-(--color-border) bg-(--color-bg-overlay) text-sm font-medium text-(--color-fg-muted)">
                  {(
                    actor.displayName?.[0] ||
                    actor.handle[0] ||
                    "?"
                  ).toUpperCase()}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium text-(--color-fg)">
                  {actor.displayName || actor.handle}
                </div>
                <div className="truncate text-xs text-(--color-fg-muted)">
                  @{actor.handle}
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}

      {!query.trim() && (
        <div className="py-16 text-center">
          <Search className="mx-auto mb-3 h-8 w-8 text-(--color-fg-muted)" />
          <p className="text-(--color-fg-muted)">{t("find-streamers")}</p>
        </div>
      )}
    </div>
  );
}
