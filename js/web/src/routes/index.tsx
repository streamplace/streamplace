import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { StreamCard } from "../components/stream/stream-card";
import useAvatars from "../hooks/use-avatars";
import { useLiveUsers } from "../hooks/use-live-users";
import { useStore } from "../lib/store";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  const { t } = useTranslation("common");
  const { data: streams, isLoading, error } = useLiveUsers();

  const allDids = streams?.map((s) => s.author.did) ?? [];
  const avatars = useAvatars(allDids);

  if (streams === undefined && isLoading) {
    return (
      <div className="mx-auto max-w-[1600px] px-4 py-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="animate-pulse rounded-xl border border-(--color-border) bg-(--color-bg-elevated)"
            >
              <div className="aspect-video rounded-t-xl bg-(--color-bg-overlay)" />
              <div className="space-y-2 p-3">
                <div className="h-4 w-3/4 rounded bg-(--color-bg-overlay)" />
                <div className="h-3 w-1/2 rounded bg-(--color-bg-overlay)" />
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[1600px] px-4 py-6">
      {error && !streams && (
        <div className="mb-8 rounded-lg border border-(--color-border) bg-(--color-bg-elevated) p-4 text-sm text-(--color-fg-muted)">
          {t("could-not-load-streams")}
        </div>
      )}

      {streams && streams.length > 0 && (
        <div className="mb-8 flex items-center gap-2.5">
          <div className="h-2.5 w-2.5 rounded-full bg-red-500 shadow-[0_0_0_4px_var(--color-bg)]" />
          <h2 className="font-display text-2xl font-semibold text-(--color-fg)">
            {t("live-now-count", { count: streams.length })}
          </h2>
        </div>
      )}

      {streams && streams.length === 0 && (
        <div className="py-20 pt-52 text-center">
          <svg
            width="100%"
            viewBox="0 0 680 360"
            role="img"
            className="mb-6 scale-400 -rotate-3"
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
                className="animate-scale animate-fade-in-staggered delay-300"
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
              <g className="translate-x-43 translate-y-20 scale-50">
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
          <h2 className="font-display text-lg font-semibold text-(--color-fg)">
            {t("no-one-streaming")}
          </h2>
          <p className="mt-1 text-(--color-fg-muted)">
            {t("check-back-later")}
          </p>
        </div>
      )}

      {streams && streams.length > 0 && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
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
        <div className="mx-auto max-w-2xl text-center">
          <h1 className="font-display text-4xl font-semibold tracking-tight">
            {t("hero-title")}
          </h1>
          <p className="mt-4 text-lg text-(--color-fg-muted)">
            {t("hero-description")}
          </p>
          <div className="mt-8 flex justify-center gap-3">
            <button
              type="button"
              onClick={() => useStore.getState().openLoginModal()}
              className="inline-flex h-10 items-center rounded-md bg-(--color-accent) px-4 font-medium text-(--color-accent-fg) transition-colors hover:bg-(--color-accent-hover)"
            >
              {t("log-in")}
            </button>
            <a
              href="https://stream.place"
              className="inline-flex h-10 items-center rounded-md border border-(--color-border) px-4 text-(--color-fg) transition-colors hover:border-(--color-border-strong)"
            >
              {t("learn-more")}
            </a>
          </div>
        </div>
      )}
    </div>
  );
}
