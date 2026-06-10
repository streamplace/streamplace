import { Link, useNavigate } from "@tanstack/react-router";
import { ArrowRight, Search, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useAvatars from "../hooks/use-avatars";
import { EMPTY_LOGIN_SEARCH } from "../lib/login-search";
import { useSession } from "../lib/session";
import { usePDSAgent, useUserProfile } from "../lib/store/hooks";
import StreamplaceSvg from "./svg/streamplace-bw";
import { SidebarTrigger } from "./ui/sidebar";

interface SearchResult {
  did: string;
  handle: string;
  displayName?: string;
}

export default function Header() {
  const { t } = useTranslation("common");
  const { state } = useSession();
  const userProfile = useUserProfile();
  useAvatars(state.status === "authenticated" ? [state.session.did] : []);

  const did = state.status === "authenticated" ? state.session.did : null;
  const avatar = userProfile?.avatar;
  const handle = userProfile?.handle;
  const displayName = userProfile?.displayName || handle;

  // --- Search ---
  const agent = usePDSAgent();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [open, setOpen] = useState(false);
  const [highlightIndex, setHighlightIndex] = useState(-1);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const resultsRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const resultDids = results.map((r) => r.did);
  const avatars = useAvatars(resultDids);

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
          limit: 8,
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

  const handleChange = (value: string) => {
    setQuery(value);
    setHighlightIndex(-1);
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    if (value.trim()) {
      setOpen(true);
      timeoutRef.current = setTimeout(() => searchActors(value), 250);
    } else {
      setResults([]);
      setOpen(false);
    }
  };

  const selectResult = useCallback(
    (actor: SearchResult) => {
      setQuery("");
      setResults([]);
      setOpen(false);
      navigate({ to: "/$user", params: { user: actor.handle } });
    },
    [navigate],
  );

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!open) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlightIndex((i) => Math.min(i + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlightIndex((i) => Math.max(i - 1, -1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (highlightIndex >= 0 && results[highlightIndex]) {
        selectResult(results[highlightIndex]);
      }
    } else if (e.key === "Escape") {
      setOpen(false);
      inputRef.current?.blur();
    }
  };

  // Close on outside click.
  useEffect(() => {
    if (!open) return;
    const onClick = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [open]);

  // Scroll highlighted item into view.
  useEffect(() => {
    if (highlightIndex < 0 || !resultsRef.current) return;
    const el = resultsRef.current.children[highlightIndex] as HTMLElement;
    el?.scrollIntoView({ block: "nearest" });
  }, [highlightIndex]);

  const clearSearch = () => {
    setQuery("");
    setResults([]);
    setOpen(false);
    inputRef.current?.focus();
  };

  return (
    <>
      <header className="flex items-center gap-4 pt-4 pb-4 py-2 h-12 bg-sidebar">
        {/* logo */}
        <div className="fixed left-4 top-2.5 z-50 flex items-center gap-2">
          <StreamplaceSvg className="w-6 h-6 invert-100" />
          <h1 className="text-lg hidden md:block">Streamplace</h1>
          <SidebarTrigger />
        </div>

        {/* search bar */}
        <div ref={containerRef} className="flex-1 flex justify-center ml-24">
          <div className="relative w-full max-w-sm">
            <div className="relative flex items-center">
              <Search className="absolute left-2.5 w-3.5 h-3.5 text-[var(--color-fg-muted)] pointer-events-none" />
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={(e) => handleChange(e.target.value)}
                onKeyDown={handleKeyDown}
                onFocus={() => {
                  if (query.trim() && results.length > 0) setOpen(true);
                }}
                placeholder={t("search-placeholder")}
                className="w-full h-8 pl-8 pr-8 text-sm rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] placeholder:text-[var(--color-fg-muted)] focus:outline-none focus:ring-1 focus:ring-[var(--color-ring)]"
              />
              {query && (
                <button
                  type="button"
                  onClick={clearSearch}
                  className="absolute right-2 p-0.5 rounded text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              )}
            </div>

            {/* results dropdown */}
            {open && (results.length > 0 || searching) && (
              <div
                ref={resultsRef}
                className="absolute top-full mt-1 left-0 right-0 z-50 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-elevated)] shadow-md overflow-hidden max-h-80 overflow-y-auto"
              >
                {searching && results.length === 0 && (
                  <div className="flex items-center justify-center py-6">
                    <div className="w-4 h-4 border-2 border-[var(--color-border)] border-t-[var(--color-accent)] rounded-full animate-spin" />
                  </div>
                )}
                {results.map((actor, i) => (
                  <button
                    key={actor.did}
                    type="button"
                    onClick={() => selectResult(actor)}
                    onMouseEnter={() => setHighlightIndex(i)}
                    className={`w-full flex items-center gap-2.5 px-3 py-2 text-left transition-colors ${
                      i === highlightIndex
                        ? "bg-[var(--color-bg-overlay)]"
                        : "hover:bg-[var(--color-bg-overlay)]"
                    }`}
                  >
                    {avatars[actor.did]?.avatar ? (
                      <img
                        src={avatars[actor.did].avatar}
                        alt=""
                        className="w-7 h-7 rounded-full bg-[var(--color-bg)] object-cover flex-shrink-0"
                      />
                    ) : (
                      <div className="w-7 h-7 rounded-full bg-[var(--color-bg-overlay)] border border-[var(--color-border)] flex items-center justify-center text-xs font-medium text-[var(--color-fg-muted)] flex-shrink-0">
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
                  </button>
                ))}
                <div className="w-full flex justify-center items-center border-t">
                  <Link
                    to="/search"
                    search={{ q: query || undefined }}
                    onClick={() => setOpen(false)}
                    className="flex items-center gap-2 text-center py-2 text-sm text-[var(--color-accent)] hover:bg-[var(--color-bg-overlay)] transition-colors"
                  >
                    {t("more-results")}{" "}
                    <ArrowRight className="w-3.5 h-3.5 inline" />
                  </Link>
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="flex items-center gap-4">
          <nav className="flex items-center gap-4">
            {did ? (
              <Link
                to="/settings/account"
                className="flex items-center gap-2 rounded-full hover:bg-[var(--color-bg-overlay)] transition-colors pl-1 pr-3 py-1"
                title={
                  displayName ? t("signed-in-as", { handle }) : t("profile")
                }
                aria-label={
                  displayName ? t("signed-in-as", { handle }) : t("profile")
                }
              >
                {avatar ? (
                  <img
                    src={avatar}
                    alt=""
                    className="w-7 h-7 rounded-full bg-(--color-bg) object-cover"
                    onError={(e) => {
                      (e.currentTarget as HTMLImageElement).style.display =
                        "none";
                    }}
                  />
                ) : (
                  <div className="w-7 h-7 rounded-full bg-[var(--color-bg-overlay)] border border-[var(--color-border)] flex items-center justify-center text-xs font-medium text-[var(--color-fg-muted)]">
                    {(displayName?.[0] || handle?.[0] || "?").toUpperCase()}
                  </div>
                )}
              </Link>
            ) : (
              <Link
                to="/login"
                search={EMPTY_LOGIN_SEARCH}
                className="text-sm font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
              >
                {t("log-in")}
              </Link>
            )}
          </nav>
        </div>
      </header>
    </>
  );
}
