import { createFileRoute, Link } from "@tanstack/react-router";
import { EMPTY_LOGIN_SEARCH } from "../lib/login-search";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  return (
    <div className="max-w-6xl mx-auto px-6 py-12">
      <div className="max-w-2xl">
        <h1 className="text-4xl font-semibold tracking-tight">
          The video layer for everything.
        </h1>
        <p className="mt-4 text-lg text-[var(--color-fg-muted)]">
          Streamplace is a streaming platform built on the AT Protocol. This is
          the web app — a true-DOM TanStack Router + React build, separate from
          the React Native mobile app, sharing data and business logic through
          the <code className="font-mono text-sm">@streamplace/core</code>{" "}
          package.
        </p>

        <div className="mt-8 flex gap-3">
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

        <div className="mt-16 grid gap-6 sm:grid-cols-2">
          <FeatureCard
            title="Shared data layer"
            body="Stores, XRPC clients, and the chat reducer live in @streamplace/core — the same code runs in the mobile app and this web build."
          />
          <FeatureCard
            title="Native web routing"
            body="TanStack Router gives you file-based routes, typed params, and search-string validation out of the box."
          />
          <FeatureCard
            title="Real-DOM components"
            body="No react-native-web, no bundle bloat. Plain React + Base UI primitives styled with Tailwind v4."
          />
          <FeatureCard
            title="ATProto OAuth"
            body="@atproto/oauth-client-browser handles sign-in, session restore, and refresh — backed by localStorage."
          />
        </div>
      </div>
    </div>
  );
}

function FeatureCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-5">
      <h3 className="font-medium">{title}</h3>
      <p className="mt-1 text-sm text-[var(--color-fg-muted)]">{body}</p>
    </div>
  );
}
