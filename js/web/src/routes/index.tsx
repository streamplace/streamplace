import { createFileRoute, Link } from "@tanstack/react-router";
import { StreamCard } from "../components/stream/stream-card";
import useAvatars from "../hooks/use-avatars";
import { EMPTY_LOGIN_SEARCH } from "../lib/login-search";
import {
  useLiveUsers,
  useLiveUsersError,
  useLiveUsersLoading,
} from "../lib/store/hooks";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  const streams = useLiveUsers();
  const loading = useLiveUsersLoading();
  const error = useLiveUsersError();

  const allDids = streams?.map((s) => s.author.did) ?? [];
  const avatars = useAvatars(allDids);

  if (streams === null && loading) {
    return (
      <div className="max-w-6xl mx-auto px-6 py-12">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-elevated)] animate-pulse"
            >
              <div className="aspect-video bg-[var(--color-bg-overlay)] rounded-t-xl" />
              <div className="p-3 space-y-2">
                <div className="h-4 bg-[var(--color-bg-overlay)] rounded w-3/4" />
                <div className="h-3 bg-[var(--color-bg-overlay)] rounded w-1/2" />
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto px-6 py-8">
      {error && streams === null && (
        <div className="mb-6 p-4 rounded-lg bg-[var(--color-bg-elevated)] border border-[var(--color-border)] text-sm text-[var(--color-fg-muted)]">
          Could not load streams. You might be offline.
        </div>
      )}

      {streams && streams.length > 0 && (
        <div className="flex items-center gap-2 mb-6">
          <div className="w-2 h-2 rounded-full bg-red-500" />
          <h2 className="text-lg font-semibold text-[var(--color-fg)]">
            {streams.length} {streams.length === 1 ? "person" : "people"} live
            now
          </h2>
        </div>
      )}

      {streams && streams.length === 0 && (
        <div className="text-center py-20">
          <div className="text-2xl mb-2">📡</div>
          <h2 className="text-lg font-semibold text-[var(--color-fg)]">
            No one is streaming right now
          </h2>
          <p className="text-sm text-[var(--color-fg-muted)] mt-1">
            Check back later?
          </p>
        </div>
      )}

      {streams && streams.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {streams.map((stream) => (
            <StreamCard
              key={stream.uri}
              stream={stream}
              avatarUrl={avatars[stream.author.did]?.avatar}
            />
          ))}
        </div>
      )}

      {streams === null && !loading && (
        <div className="max-w-2xl mx-auto text-center">
          <h1 className="text-4xl font-semibold tracking-tight">
            The video layer for everything.
          </h1>
          <p className="mt-4 text-lg text-[var(--color-fg-muted)]">
            Streamplace is a streaming platform built on the AT Protocol.
          </p>
          <div className="mt-8 flex gap-3 justify-center">
            <Link
              to="/login"
              search={EMPTY_LOGIN_SEARCH}
              className="inline-flex items-center px-4 h-10 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-[var(--color-accent-fg)] font-medium transition-colors"
            >
              Log in
            </Link>
            <a
              href="https://stream.place"
              className="inline-flex items-center px-4 h-10 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-[var(--color-fg)] transition-colors"
            >
              Learn more
            </a>
          </div>
        </div>
      )}
    </div>
  );
}
