import { createFileRoute, Link } from "@tanstack/react-router";
import { useState } from "react";
import { EMPTY_LOGIN_SEARCH } from "../lib/login-search";
import { useSession } from "../lib/session";
import {
  clearStoredServerUrl,
  getStoredServerUrl,
  getStreamplaceUrl,
  setStoredServerUrl,
} from "../lib/streamplace-url";

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
});

function isValidServerUrl(value: string): boolean {
  if (!value) return false;
  try {
    const u = new URL(value);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

function sourceLabel(): string {
  if (getStoredServerUrl()) return "runtime override (localStorage)";
  const fromEnv = import.meta.env["VITE_STREAMPLACE_URL"];
  if (typeof fromEnv === "string" && fromEnv.length > 0) {
    return "build-time (VITE_STREAMPLACE_URL)";
  }
  return "window.location.origin (default)";
}

function SettingsPage() {
  const { state, signOut } = useSession();
  const [draft, setDraft] = useState<string>(() => {
    return getStoredServerUrl() ?? getStreamplaceUrl();
  });
  const [saved, setSaved] = useState(false);

  const trimmed = draft.trim();
  const valid = isValidServerUrl(trimmed);
  const dirty = trimmed !== getStreamplaceUrl();

  const onSave = () => {
    if (!valid) return;
    setStoredServerUrl(trimmed);
    window.location.reload();
  };

  const onReset = () => {
    clearStoredServerUrl();
    setDraft(getStreamplaceUrl());
    setSaved(true);
  };

  return (
    <div className="max-w-2xl mx-auto px-6 py-10 space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold">Settings</h1>
        <p className="text-sm text-[var(--color-fg-muted)]">
          Configure which Streamplace server this web app talks to.
        </p>
      </header>

      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          Server
        </h2>

        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 space-y-3">
          <div className="text-xs text-[var(--color-fg-muted)]">
            Current: <span className="font-mono">{getStreamplaceUrl()}</span>
            <span className="ml-2 text-[var(--color-fg-subtle)]">
              ({sourceLabel()})
            </span>
          </div>

          <label className="block">
            <span className="block text-sm font-medium mb-1">Server URL</span>
            <input
              type="url"
              value={draft}
              onChange={(e) => {
                setDraft(e.target.value);
                setSaved(false);
              }}
              placeholder="http://127.0.0.1:38080"
              spellCheck={false}
              autoComplete="off"
              className="w-full h-10 px-3 rounded-md bg-[var(--color-bg)] border border-[var(--color-border)] focus:border-[var(--color-accent)] focus:outline-none text-sm font-mono"
            />
            <span className="block text-xs text-[var(--color-fg-subtle)] mt-1">
              Must be http:// or https://. Changes are saved to localStorage and
              take effect after a reload.
            </span>
          </label>

          {draft.length > 0 && !valid && (
            <div className="text-xs text-[var(--color-danger)]">
              That doesn't look like a valid URL.
            </div>
          )}

          <div className="flex items-center gap-2 pt-1">
            <button
              type="button"
              onClick={onSave}
              disabled={!valid || !dirty}
              className="h-9 px-4 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed text-[var(--color-accent-fg)] text-sm font-medium"
            >
              Save & reload
            </button>
            <button
              type="button"
              onClick={onReset}
              disabled={!getStoredServerUrl()}
              className="h-9 px-4 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] disabled:opacity-50 disabled:cursor-not-allowed text-sm"
            >
              Reset to default
            </button>
            {saved && (
              <span className="text-xs text-[var(--color-fg-muted)]">
                Saved.
              </span>
            )}
          </div>
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          Account
        </h2>

        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 flex items-center justify-between">
          <div className="text-sm">
            {state.status === "authenticated" ? (
              <>
                Signed in as{" "}
                <span className="font-mono">{state.session.did}</span>
              </>
            ) : state.status === "loading" ? (
              <span className="text-[var(--color-fg-muted)]">Checking…</span>
            ) : (
              <span className="text-[var(--color-fg-muted)]">
                Not signed in
              </span>
            )}
          </div>
          {state.status === "authenticated" ? (
            <button
              type="button"
              onClick={() => {
                void signOut();
              }}
              className="h-9 px-4 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-sm"
            >
              Sign out
            </button>
          ) : (
            <Link
              to="/login"
              search={EMPTY_LOGIN_SEARCH}
              className="h-9 inline-flex items-center px-4 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-[var(--color-accent-fg)] text-sm font-medium"
            >
              Log in
            </Link>
          )}
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium uppercase tracking-wide text-[var(--color-fg-muted)]">
          About
        </h2>
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 text-sm text-[var(--color-fg-muted)]">
          Streamplace web · v0.0.0
        </div>
      </section>
    </div>
  );
}
