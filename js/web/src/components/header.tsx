import { Link, useNavigate } from "@tanstack/react-router";
import { ArrowRight, Search, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
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
        const response = await agent.client.call(
          place.stream.live.searchActorsTypeahead,
          {
            q,
            limit: 8,
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
      <header className="bg-sidebar flex h-12 items-center gap-4 py-2 pt-4 pb-4">
        {/* logo */}
        <div className="fixed top-2.5 left-4 z-50 flex items-center gap-2">
          <StreamplaceSvg className="h-6 w-6 invert-100" />
          <h1 className="hidden text-lg md:block">Streamplace</h1>
          <SidebarTrigger />
        </div>

        {/* search bar */}
        <div ref={containerRef} className="ml-24 flex flex-1 justify-center">
          <div className="relative w-full max-w-sm">
            <div className="relative flex items-center">
              <Search className="pointer-events-none absolute left-2.5 h-3.5 w-3.5 text-(--color-fg-muted)" />
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
                className="focus:ring-ring h-8 w-full rounded-md border border-(--color-border) bg-(--color-bg) pr-8 pl-8 text-sm text-(--color-fg) placeholder:text-(--color-fg-muted) focus:ring-1 focus:outline-none"
              />
              {query && (
                <button
                  type="button"
                  onClick={clearSearch}
                  className="absolute right-2 rounded p-0.5 text-(--color-fg-muted) hover:text-(--color-fg)"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>

            {/* results dropdown */}
            {open && (results.length > 0 || searching) && (
              <div
                ref={resultsRef}
                className="absolute top-full right-0 left-0 z-50 mt-1 max-h-80 overflow-hidden overflow-y-auto rounded-md border border-(--color-border) bg-(--color-bg-elevated) shadow-md"
              >
                {searching && results.length === 0 && (
                  <div className="flex items-center justify-center py-6">
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-(--color-border) border-t-(--color-accent)" />
                  </div>
                )}
                {results.map((actor, i) => (
                  <button
                    key={actor.did}
                    type="button"
                    onClick={() => selectResult(actor)}
                    onMouseEnter={() => setHighlightIndex(i)}
                    className={`flex w-full items-center gap-2.5 px-3 py-2 text-left transition-colors ${
                      i === highlightIndex
                        ? "bg-(--color-bg-overlay)"
                        : "hover:bg-(--color-bg-overlay)"
                    }`}
                  >
                    {avatars[actor.did]?.avatar ? (
                      <img
                        src={avatars[actor.did].avatar}
                        alt=""
                        className="h-7 w-7 shrink-0 rounded-full bg-(--color-bg) object-cover"
                      />
                    ) : (
                      <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-(--color-border) bg-(--color-bg-overlay) text-xs font-medium text-(--color-fg-muted)">
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
                  </button>
                ))}
                <div className="flex w-full items-center justify-center border-t">
                  <Link
                    to="/search"
                    search={{ q: query || undefined }}
                    onClick={() => setOpen(false)}
                    className="flex items-center gap-2 py-2 text-center text-sm text-(--color-accent) transition-colors hover:bg-(--color-bg-overlay)"
                  >
                    {t("more-results")}{" "}
                    <ArrowRight className="inline h-3.5 w-3.5" />
                  </Link>
                </div>
              </div>
            )}
          </div>
        </div>

        <UserProfile
          did={did}
          displayName={displayName}
          t={t}
          handle={handle}
          avatar={avatar}
        />
      </header>
    </>
  );
}

export function UserProfile({
  did,
  displayName,
  t,
  handle,
  avatar,
}: {
  did: string | null;
  displayName: string | undefined;
  t: any;
  handle: string | undefined;
  avatar: string | undefined;
}) {
  return (
    <div className="flex items-center gap-4">
      <nav className="flex items-center gap-4">
        {did ? (
          <Link
            to="/settings/account"
            className="flex items-center gap-2 rounded-full py-1 pr-3 pl-1 transition-colors hover:bg-(--color-bg-overlay)"
            title={displayName ? t("signed-in-as", { handle }) : t("profile")}
            aria-label={
              displayName ? t("signed-in-as", { handle }) : t("profile")
            }
          >
            {avatar ? (
              <img
                src={avatar}
                alt=""
                className="h-7 w-7 rounded-full bg-(--color-bg) object-cover"
                onError={(e) => {
                  (e.currentTarget as HTMLImageElement).style.display = "none";
                }}
              />
            ) : (
              <div className="flex h-7 w-7 items-center justify-center rounded-full border border-(--color-border) bg-(--color-bg-overlay) text-xs font-medium text-(--color-fg-muted)">
                {(displayName?.[0] || handle?.[0] || "?").toUpperCase()}
              </div>
            )}
          </Link>
        ) : (
          <Link
            to="/login"
            search={EMPTY_LOGIN_SEARCH}
            className="text-sm font-medium text-(--color-fg-muted) transition-colors hover:text-(--color-fg)"
          >
            {t("log-in")}
          </Link>
        )}
      </nav>
    </div>
  );
}
