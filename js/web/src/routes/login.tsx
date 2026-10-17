import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { consumeAuthReturnPath } from "../lib/auth-return";
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
  const { t } = useTranslation("common");
  const { state, signIn } = useSession();
  const navigate = useNavigate();
  const search = Route.useSearch();
  const [handle, setHandle] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // This route doubles as the OAuth callback target. In the popup case,
  // client.initCallback (called by BlueskyProvider's oauthCallback) sends
  // the result back to the opener via BroadcastChannel, then calls
  // window.close() to close the popup. The library also re-throws
  // LoginContinuedInParentWindowError, which our oauthCallback catch
  // handles by setting authStatus to "loggedOut". In the direct /login
  // case (user landed here directly, or popup-blocked fallback),
  // initCallback returns the session, state becomes "authenticated",
  // and we navigate to / via the effect further down.
  const isCallbackInFlight =
    Boolean(search.code) ||
    Boolean(search.error) ||
    Boolean(state.status === "loading");

  useEffect(() => {
    if (search.error) {
      setError(t("sign-in-failed"));
      return;
    }
    // In the popup case state.status never reaches "authenticated"
    // (the session lives in the opener, not the popup). The library's
    // initCallback closes the popup via window.close() after sending
    // the result to the parent. The navigate fires only for the direct
    // /login case.
    if (state.status === "authenticated" && !window.opener) {
      const returnPath = consumeAuthReturnPath();
      if (returnPath) {
        window.location.replace(returnPath);
        return;
      }
      navigate({ to: "/" });
    }
  }, [state.status, search.error, search.errorDescription, navigate]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      // /login is the full-page route; always do a full-page redirect
      // to the PDS rather than opening a popup. This is the reliable
      // fallback for users who landed here directly, or who got bounced
      // here by the modal's popup-blocker detection.
      await signIn(handle.trim(), "redirect");
    } catch (err) {
      console.error("Sign-in failed:", err);
      setError(t("sign-in-failed"));
      setSubmitting(false);
    }
  };

  if (isCallbackInFlight && state.status !== "authenticated") {
    return (
      <div className="mx-auto max-w-md px-6 py-16 text-center">
        <p className="text-(--color-fg-muted)">{t("completing-sign-in")}</p>
      </div>
    );
  }

  if (state.status === "authenticated") {
    return (
      <div className="mx-auto max-w-md px-6 py-16 text-center">
        <h1 className="font-display text-2xl font-semibold">
          {t("already-logged-in")}
        </h1>
        <p className="mt-2 text-(--color-fg-muted)">
          {t("signed-in-as-code", { handle: state.session.sub })}
        </p>
        <Button
          type="button"
          onClick={() => navigate({ to: "/" })}
          size="lg"
          className="mt-6"
        >
          {t("go-home")}
        </Button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-md px-6 py-16">
      <h1 className="font-display text-2xl font-semibold">{t("log-in")}</h1>
      <p className="mt-2 text-(--color-fg-muted)">{t("sign-in-description")}</p>

      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <label className="block" htmlFor="login-handle">
          <span className="text-sm text-(--color-fg-muted)">
            {t("handle-label")}
          </span>
          <Input
            id="login-handle"
            type="text"
            value={handle}
            onChange={(e) => setHandle(e.target.value)}
            placeholder="you.bsky.social"
            autoComplete="username"
            required
            className="bg-card mt-1 h-10"
          />
        </label>

        {error && (
          <p className="text-sm text-(--color-danger)" role="alert">
            {error}
          </p>
        )}

        <Button
          type="submit"
          disabled={submitting || state.status === "loading"}
          size="lg"
          className="w-full"
        >
          {submitting ? t("redirecting") : t("continue")}
        </Button>
      </form>
    </div>
  );
}
