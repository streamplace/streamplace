import { createFileRoute, Link } from "@tanstack/react-router";
import { StreamCard } from "../components/stream/stream-card";
import useAvatars from "../hooks/use-avatars";
import { useLiveUsers } from "../hooks/use-live-users";
import { EMPTY_LOGIN_SEARCH } from "../lib/login-search";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  const { data: streams, isLoading, error } = useLiveUsers();

  const allDids = streams?.map((s) => s.author.did) ?? [];
  const avatars = useAvatars(allDids);

  if (streams === undefined && isLoading) {
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
      {error && !streams && (
        <div className="mb-6 p-4 rounded-lg bg-[var(--color-bg-elevated)] border border-[var(--color-border)] text-sm text-[var(--color-fg-muted)]">
          Could not load streams. You might be offline.
        </div>
      )}

      {streams && streams.length > 0 && (
        <div className="flex items-center gap-2 mb-6">
          <div className="w-2 h-2 rounded-full bg-red-500" />
          <h2 className="text-lg font-semibold font-display text-[var(--color-fg)]">
            {streams.length} {streams.length === 1 ? "person" : "people"} live
            now
          </h2>
        </div>
      )}

      {streams && streams.length === 0 && (
        <div className="text-center py-20 pt-52">
          <svg
            width="100%"
            viewBox="0 0 680 360"
            role="img"
            className="-rotate-3 scale-400 mb-6"
          >
            <g
              stroke="currentColor"
              stroke-width="0.5"
              fill="none"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <g
                opacity="0.35"
                className="animate-scale delay-300 animate-fade-in-staggered"
              >
                <circle
                  style={{ "--anim-order": 0 } as any}
                  className="animate-fade-in-staggered"
                  cx="60"
                  cy="50"
                  r="1.4"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="140"
                  cy="120"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="220"
                  cy="40"
                  r="1.2"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="320"
                  cy="90"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="420"
                  cy="35"
                  r="1.5"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="520"
                  cy="70"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="610"
                  cy="45"
                  r="1.3"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="90"
                  cy="220"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="50"
                  cy="310"
                  r="1.2"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="170"
                  cy="300"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="280"
                  cy="330"
                  r="1.4"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="480"
                  cy="320"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="560"
                  cy="250"
                  r="1.2"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="630"
                  cy="300"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="380"
                  cy="160"
                  r="0.9"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="600"
                  cy="160"
                  r="1"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="110"
                  cy="160"
                  r="0.9"
                ></circle>
                <circle
                  className="animate-fade-in-staggered"
                  cx="250"
                  cy="200"
                  r="0.8"
                ></circle>
              </g>
              <g
                opacity="0.35"
                className="animate-scale animate-fade-in-staggered"
              >
                <path
                  style={{ "--anim-order": 0 } as any}
                  className="animate-fade-in-staggered"
                  d="M90 70 l0 12 M84 76 l12 0"
                ></path>
                <path
                  style={{ "--anim-order": 1 } as any}
                  className="animate-fade-in-staggered"
                  d="M590 60 l0 10 M585 65 l10 0"
                ></path>
                <circle
                  style={{ "--anim-order": 2 } as any}
                  className="animate-fade-in-staggered"
                  cx="90"
                  cy="290"
                  r="2.5"
                ></circle>
                <circle
                  style={{ "--anim-order": 3 } as any}
                  className="animate-fade-in-staggered"
                  cx="580"
                  cy="300"
                  r="2.5"
                ></circle>
                <circle
                  style={{ "--anim-order": 4 } as any}
                  className="animate-fade-in-staggered"
                  cx="610"
                  cy="180"
                  r="2"
                ></circle>
                <circle
                  style={{ "--anim-order": 5 } as any}
                  className="animate-fade-in-staggered"
                  cx="70"
                  cy="190"
                  r="2"
                ></circle>
              </g>
              <g className="scale-50 translate-x-43 translate-y-20">
                <g className="animate-in-hubble">
                  <g id="scope" className="animate-scope-rock" stroke-width="2">
                    <g transform="rotate(90, 230, 180)">
                      <rect
                        x="178"
                        y="152"
                        width="104"
                        height="56"
                        rx="6"
                      ></rect>
                      <line x1="204" y1="152" x2="204" y2="208"></line>
                      <line x1="230" y1="120" x2="230" y2="208"></line>
                      <line x1="256" y1="152" x2="256" y2="208"></line>
                    </g>
                    <g transform="rotate(90, 450, 180)">
                      <rect
                        x="398"
                        y="152"
                        width="104"
                        height="56"
                        rx="6"
                      ></rect>
                      <line x1="424" y1="152" x2="424" y2="208"></line>
                      <line x1="450" y1="152" x2="450" y2="248"></line>
                      <line x1="476" y1="152" x2="476" y2="208"></line>
                    </g>

                    <line x1="282" y1="180" x2="296" y2="180"></line>
                    <line x1="384" y1="180" x2="398" y2="180"></line>

                    <path d="M296 134 h88 a14 14 0 0 1 14 14 v64 a14 14 0 0 1 -14 14 h-88 a14 14 0 0 1 -14 -14 v-64 a14 14 0 0 1 14 -14 z"></path>
                    <line x1="282" y1="156" x2="398" y2="156"></line>
                    <line x1="282" y1="204" x2="398" y2="204"></line>
                    <circle cx="340" cy="180" r="13"></circle>
                    <circle cx="340" cy="180" r="5"></circle>
                    <line x1="340" y1="134" x2="340" y2="106"></line>
                    <path d="M331 106 h18"></path>
                    <circle cx="340" cy="98" r="6"></circle>
                    <path d="M312 222 l-10 16 M368 222 l10 16"></path>
                  </g>
                </g>
              </g>
            </g>
          </svg>
          <h2 className="text-lg font-semibold font-display text-[var(--color-fg)]">
            No one is streaming right now
          </h2>
          <p className="text-[var(--color-fg-muted)] mt-1">Check back later?</p>
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

      {!streams && !isLoading && !error && (
        <div className="max-w-2xl mx-auto text-center">
          <h1 className="text-4xl font-semibold font-display tracking-tight">
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
