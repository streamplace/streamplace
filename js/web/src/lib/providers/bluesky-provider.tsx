// Bluesky/ATProto lifecycle provider. Port of
// js/app/features/bluesky/blueskyProvider.tsx, minus the React
// Navigation and Expo Linking bits (web uses URL search params).
//
// Responsibilities:
//   1. On mount, check whether the current URL is an OAuth callback
//      (has `?code=` or `?error=`). If so, hand it to the store's
//      `oauthCallback` action; otherwise call `loadOAuthClient` to
//      restore an existing session.
//   2. Poll every 5s while the slice is not yet ready. The app uses
//      the same hack — it keeps retrying `loadOAuthClient` because
//      the OAuth client restores from localStorage async.
//   3. Pull the authenticated user's profile when we have a session
//      but no profile cached.
//   4. Run useBlueskyNotifications to surface login errors as toasts.
import { ReactNode, useEffect } from "react";
import { useBlueskyNotifications } from "../../hooks/use-bluesky-notifications";
import { useStore } from "../store";
import { useIsReady, useUserProfile } from "../store/hooks";

export default function BlueskyProvider({ children }: { children: ReactNode }) {
  const loadOAuthClient = useStore((state) => state.loadOAuthClient);
  const oauthCallback = useStore((state) => state.oauthCallback);
  const getProfile = useStore((state) => state.getProfile);
  const oauthSession = useStore((state) => state.oauthSession);
  const isReady = useIsReady();
  const userProfile = useUserProfile();

  useBlueskyNotifications();

  useEffect(() => {
    if (typeof window === "undefined") return;
    const url = window.location.href;
    if (url.includes("code=") || url.includes("error=")) {
      void oauthCallback(url);
    } else {
      void loadOAuthClient();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (isReady) return;
    const handle = setInterval(() => {
      void loadOAuthClient();
    }, 5000);
    return () => clearInterval(handle);
  }, [isReady, loadOAuthClient]);

  useEffect(() => {
    if (oauthSession && !userProfile) {
      void getProfile(oauthSession.did);
    }
  }, [oauthSession, userProfile, getProfile]);

  return <>{children}</>;
}
