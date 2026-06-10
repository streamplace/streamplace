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
        const response = await agent.place.stream.live.searchActorsTypeahead({
          q,
          limit: 20,
        });
        setResults(
          response.data.actors.map((a: any) => ({
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
    <div className="max-w-2xl mx-auto w-full px-4 py-6">
      <div className="relative mb-6">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-fg-muted)]" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => handleChange(e.target.value)}
          placeholder={t("search-for-streamers")}
          className="w-full h-10 pl-9 pr-4 rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-muted)] focus:outline-none focus:ring-1 focus:ring-[var(--color-ring)]"
        />
      </div>

      {searching && results.length === 0 && (
        <div className="flex justify-center py-12">
          <div className="w-5 h-5 border-2 border-[var(--color-border)] border-t-[var(--color-accent)] rounded-full animate-spin" />
        </div>
      )}

      {!searching && query.trim() && results.length === 0 && (
        <div className="text-center py-12">
          <p className="text-[var(--color-fg-muted)]">
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
              className="flex items-center gap-3 p-3 rounded-lg hover:bg-[var(--color-bg-elevated)] transition-colors"
            >
              {avatars[actor.did]?.avatar ? (
                <img
                  src={avatars[actor.did].avatar}
                  alt=""
                  className="w-10 h-10 rounded-full bg-[var(--color-bg)] object-cover"
                />
              ) : (
                <div className="w-10 h-10 rounded-full bg-[var(--color-bg-overlay)] border border-[var(--color-border)] flex items-center justify-center text-sm font-medium text-[var(--color-fg-muted)]">
                  {(
                    actor.displayName?.[0] ||
                    actor.handle[0] ||
                    "?"
                  ).toUpperCase()}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-[var(--color-fg)] truncate">
                  {actor.displayName || actor.handle}
                </div>
                <div className="text-xs text-[var(--color-fg-muted)] truncate">
                  @{actor.handle}
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}

      {!query.trim() && (
        <div className="text-center py-16">
          <Search className="w-8 h-8 mx-auto text-[var(--color-fg-muted)] mb-3" />
          <p className="text-[var(--color-fg-muted)]">{t("find-streamers")}</p>
        </div>
      )}
    </div>
  );
}
