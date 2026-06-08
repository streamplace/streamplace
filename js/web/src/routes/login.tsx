import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useSession } from "../lib/session";

export const Route = createFileRoute("/login")({
  // The /login route doubles as the OAuth callback target.
  validateSearch: (
    search: Record<string, unknown>,
  ): {
    code: string | undefined;
    state: string | undefined;
    iss: string | undefined;
    error: string | undefined;
    errorDescription: string | undefined;
  } => ({
    code: typeof search.code === "string" ? search.code : undefined,
    state: typeof search.state === "string" ? search.state : undefined,
    iss: typeof search.iss === "string" ? search.iss : undefined,
    error: typeof search.error === "string" ? search.error : undefined,
    errorDescription:
      typeof search.error_description === "string"
        ? search.error_description
        : undefined,
  }),
  component: LoginPage,
});

function LoginPage() {
  const { state, signIn } = useSession();
  const navigate = useNavigate();
  const search = Route.useSearch();
  const [handle, setHandle] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // This route doubles as the OAuth callback target.
  const isCallbackInFlight =
    Boolean(search.code) ||
    Boolean(search.error) ||
    Boolean(state.status === "loading");

  useEffect(() => {
    if (search.error) {
      setError(
        search.errorDescription
          ? `${search.error}: ${search.errorDescription}`
          : search.error,
      );
      return;
    }
    if (state.status === "authenticated") {
      navigate({ to: "/" });
    }
  }, [state.status, search.error, search.errorDescription, navigate]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await signIn(handle.trim());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign-in failed");
      setSubmitting(false);
    }
  };

  if (isCallbackInFlight && state.status !== "authenticated") {
    return (
      <div className="max-w-md mx-auto px-6 py-16 text-center">
        <p className="text-[var(--color-fg-muted)]">Completing sign-in…</p>
      </div>
    );
  }

  if (state.status === "authenticated") {
    return (
      <div className="max-w-md mx-auto px-6 py-16 text-center">
        <h1 className="text-2xl font-semibold">You're already logged in.</h1>
        <p className="mt-2 text-[var(--color-fg-muted)]">
          Signed in as <code className="font-mono">{state.session.sub}</code>
        </p>
        <button
          type="button"
          onClick={() => navigate({ to: "/" })}
          className="mt-6 inline-flex items-center px-4 h-10 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-[var(--color-accent-fg)] font-medium"
        >
          Go home
        </button>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto px-6 py-16">
      <h1 className="text-2xl font-semibold">Log in</h1>
      <p className="mt-2 text-[var(--color-fg-muted)]">
        Sign in with your Bluesky handle. You'll be redirected to your PDS to
        authorize this app.
      </p>

      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <label className="block">
          <span className="text-sm text-[var(--color-fg-muted)]">Handle</span>
          <input
            type="text"
            value={handle}
            onChange={(e) => setHandle(e.target.value)}
            placeholder="you.bsky.social"
            autoComplete="username"
            required
            className="mt-1 w-full h-10 px-3 rounded-md bg-[var(--color-bg-elevated)] border border-[var(--color-border)] focus:border-[var(--color-accent)] focus:outline-none transition-colors"
          />
        </label>

        {error && <p className="text-sm text-[var(--color-danger)]">{error}</p>}

        <button
          type="submit"
          disabled={submitting || state.status === "loading"}
          className="w-full h-10 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed text-[var(--color-accent-fg)] font-medium transition-colors"
        >
          {submitting ? "Redirecting…" : "Continue"}
        </button>
      </form>
    </div>
  );
}
