// Modal that hosts the handle-entry form for Bluesky/ATProto login.
// Opened via openLoginModal() from any "log in" entry point that
// doesn't want to navigate the main window off the current page (the
// chat input being the motivating case: the stream player would stop).
//
// Flow:
//   1. User types a handle. Once 3+ chars, useActorTypeahead hits the
//      public Bluesky API and offers a ghost-completion suggestion +
//      avatar preview inline in the input. Tab / ArrowRight / Space
//      accepts the suggestion.
//   2. User clicks "Continue" → signIn(handle) opens a popup for the
//      PDS OAuth round-trip. The modal watches the session and closes
//      itself once authenticated.
//   3. User clicks "Sign Up" → modal closes and the PDS host selector
//      opens. The PDS handles account creation during the OAuth flow.
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { saveAuthReturnPath } from "@/lib/auth-return";
import { useSession } from "@/lib/session";
import { useStore } from "@/lib/store";
import { useActorTypeahead, type Actor } from "@streamplace/core";
import { AtSign, CornerDownLeft } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

const MIN_TYPEAHEAD_LENGTH = 3;

export function LoginModal() {
  const showLoginModal = useStore((s) => s.showLoginModal);
  const closeLoginModal = useStore((s) => s.closeLoginModal);
  const openPdsModal = useStore((s) => s.openPdsModal);
  const loginError = useStore((s) => s.loginState.error);
  const setLoginError = useStore((s) => s.setLoginError);
  const { state, signIn } = useSession();
  const { t } = useTranslation("common");
  const [handle, setHandle] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const { actors } = useActorTypeahead(handle);

  // Filter typeahead matches to handles that start with what the user
  // typed. The API can return partial matches with extra characters
  // mid-string; we want prefix matches only.
  const suggestion = useMemo<Actor | null>(() => {
    if (actors.length === 0) return null;
    if (handle.length < MIN_TYPEAHEAD_LENGTH) return null;
    const match = actors.find(
      (actor) =>
        actor.handle.toLowerCase().startsWith(handle.toLowerCase()) &&
        actor.handle.toLowerCase() !== handle.toLowerCase(),
    );
    return match ?? null;
  }, [actors, handle]);

  // The portion of the suggested handle the user hasn't typed yet,
  // shown as ghost text inside the input.
  const completionText = useMemo(() => {
    if (!suggestion) return null;
    return suggestion.handle.slice(handle.length);
  }, [suggestion, handle]);

  const acceptSuggestion = () => {
    if (suggestion) {
      setHandle(suggestion.handle);
    }
  };

  // Reset local form state and any stale error whenever the modal opens.
  useEffect(() => {
    if (!showLoginModal) return;
    setHandle("");
    setSubmitting(false);
    setLoginError(null);
  }, [showLoginModal, setLoginError]);

  // Auto-close once the popup completes and the opener is authenticated.
  useEffect(() => {
    if (!submitting) return;
    if (state.status !== "authenticated") return;
    setSubmitting(false);
    closeLoginModal();
  }, [submitting, state.status, closeLoginModal]);

  // If an error lands while we're waiting on the popup, drop back to the
  // form so the user sees the message and can retry. The library's
  // signInPopup rejects on any failure (user denial, state mismatch,
  // network, etc.) and login() routes that to loginState.error.
  useEffect(() => {
    if (!loginError) return;
    if (!submitting) return;
    setSubmitting(false);
  }, [loginError, submitting]);

  // Bounce to the full-page /login route. Used both by the
  // popup-blocked branch of onSubmit and by the manual fallback
  // button on the "Completing sign-in…" view (some browsers /
  // extensions silently block popups, so the auto-detection never
  // fires and the user needs a way out). Clear any pending
  // notification first so an abandoned signInPopup's eventual
  // library-internal timeout doesn't surface a toast on the new
  // page.
  const goToLoginPage = () => {
    saveAuthReturnPath(
      `${window.location.pathname}${window.location.search}${window.location.hash}`,
    );
    closeLoginModal();
    useStore.getState().clearNotification();
    window.location.href = "/login";
  };

  const onSignUp = () => {
    closeLoginModal();
    openPdsModal();
  };

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = handle.trim();
    if (!trimmed) return;

    setSubmitting(true);

    // Detect popup-blocked by tracking whether window.open returned a
    // handle. The OAuth library's signInPopup() calls
    // window.open('about:blank', …) synchronously early in the function
    // (see @atproto/oauth-client-browser's browser-oauth-client.js), so
    // a null return means the browser blocked the popup. The previous
    // version raced signIn() against a 4-second timeout, which
    // incorrectly treated any user who took >4s to authorize on the PDS
    // as popup-blocked and bounced them to the full-page /login route.
    const originalOpen = window.open;
    let popup: Window | null = null;
    window.open = (...args) => {
      const result = originalOpen.apply(window, args);
      if (result) popup = result;
      return result;
    };

    try {
      await signIn(trimmed);
    } catch {
      if (!popup) {
        // Popup never opened; browser blocked it. Fall back to the
        // full-page /login route, which always does a full-page
        // redirect and works regardless of popup settings.
        goToLoginPage();
        return;
      }
      // Popup opened but the OAuth flow failed (user denied, error,
      // etc.). The login() action routes the error to loginState.error.
      setSubmitting(false);
    } finally {
      window.open = originalOpen;
    }
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Tab" && completionText) {
      e.preventDefault();
      acceptSuggestion();
    } else if (e.key === "ArrowRight" && completionText) {
      const input = e.currentTarget;
      if (input.selectionStart === handle.length) {
        e.preventDefault();
        acceptSuggestion();
      }
    } else if (e.key === " " && completionText) {
      e.preventDefault();
      acceptSuggestion();
    }
  };

  return (
    <Dialog
      open={showLoginModal}
      onOpenChange={(open) => {
        if (!open) closeLoginModal();
      }}
    >
      <DialogContent>
        <div className="mb-2 flex flex-col gap-2">
          <DialogTitle>{t("log-in")}</DialogTitle>
          <DialogDescription>{t("sign-in-description")}</DialogDescription>
        </div>
        {submitting ? (
          <div className="space-y-3 py-6 text-center">
            <p className="text-sm text-(--color-fg-muted)">
              {t("completing-sign-in")}
            </p>
            <Button
              type="button"
              variant="link"
              onClick={goToLoginPage}
              className="h-auto p-0 text-sm"
            >
              {t("use-login-page")}
            </Button>
          </div>
        ) : (
          <form onSubmit={onSubmit} className="space-y-2">
            <div>
              <label
                htmlFor="login-handle"
                className="block text-sm text-(--color-fg-muted)"
              >
                {t("handle-label")}
              </label>
              <div className="relative mt-1">
                {/* Avatar / @ icon, absolutely positioned over the
                    left side of the input. */}
                <div className="pointer-events-none absolute top-1/2 left-2.5 z-10 -translate-y-1/2">
                  {suggestion?.avatar ? (
                    <img
                      src={suggestion.avatar}
                      alt=""
                      className="size-7 rounded-full object-cover"
                    />
                  ) : (
                    <div className="flex size-7 items-center justify-center rounded-full bg-(--color-bg-overlay)">
                      <AtSign className="size-4 text-(--color-fg-muted)" />
                    </div>
                  )}
                </div>

                {/* Ghost completion text. Rendered in the same row as
                    the input text (same font, same padding) so it
                    appears to extend the user's typing. The input's
                    own text overlays it via caret + selection. */}
                {completionText && suggestion?.handle !== handle && (
                  <div className="pointer-events-none absolute top-0 left-0 flex h-10 items-center pl-12 text-base text-(--color-fg-subtle) opacity-50">
                    <span>{handle}</span>
                    <span>{completionText}</span>
                    <CornerDownLeft className="ml-1 size-3.5" />
                  </div>
                )}

                <input
                  id="login-handle"
                  type="text"
                  value={handle}
                  onChange={(e) =>
                    setHandle(
                      e.target.value
                        .toLowerCase()
                        .replace(/[\u202A\u202C\u200E\u200F\u2066-\u2069]/g, "")
                        .trim(),
                    )
                  }
                  onKeyDown={onKeyDown}
                  placeholder="you.bsky.social"
                  autoComplete="username"
                  autoCapitalize="none"
                  autoCorrect="false"
                  spellCheck={false}
                  required
                  autoFocus
                  className="h-10 w-full rounded-md border border-(--color-border) bg-(--color-bg-elevated) pr-3 pl-12 transition-colors focus:border-(--color-accent) focus:outline-none"
                />
              </div>
            </div>

            {loginError && (
              <p className="text-sm text-(--color-danger)">{loginError}</p>
            )}

            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="ghost"
                onClick={onSignUp}
                disabled={submitting}
              >
                {t("sign-up")}
              </Button>
              <Button type="submit" disabled={!handle.trim() || submitting}>
                {t("continue")}
              </Button>
            </div>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
