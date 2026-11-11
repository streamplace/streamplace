import { useNavigation } from "@react-navigation/native";
import { storage } from "@streamplace/components";
import { useLinkingURL } from "expo-linking";
import { useEffect, useState } from "react";
import { Platform } from "react-native";
import { useStore } from "store";
import { useIsReady, useOAuthSession, useUserProfile } from "store/hooks";
import { navigateToRoute } from "utils/navigation";
import { getDesktopAuth } from "./desktop-auth";

export default function BlueskyProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const loadOAuthClient = useStore((state) => state.loadOAuthClient);
  const oauthCallback = useStore((state) => state.oauthCallback);
  const getProfile = useStore((state) => state.getProfile);
  const closeLoginModal = useStore((state) => state.closeLoginModal);
  const setReturnRoute = useStore((state) => state.setReturnRoute);
  const navigation = useNavigation();
  const isReady = useIsReady();

  useEffect(() => {
    loadOAuthClient();

    // load return route from storage on mount
    if (Platform.OS !== "web") {
      return;
    }
    storage.getItem("returnRoute").then((stored) => {
      if (stored) {
        // if we're on native, delete it as we don't close the app during sign in
        if (Platform.OS !== "web") storage.removeItem("returnRoute");
        try {
          const route = JSON.parse(stored);
          console.log("Loaded returnRoute from storage:", route);
          setReturnRoute(route);
        } catch (e) {
          console.error("Failed to parse returnRoute from storage", e);
        }
      }
    });
  }, []);

  useEffect(() => {
    if (!isReady) {
      const handle = setInterval(() => {
        loadOAuthClient();
      }, 5000);
      return () => clearInterval(handle);
    }
  }, [isReady]);

  const oauthSession = useOAuthSession();
  const userProfile = useUserProfile();
  const returnRoute = useStore((state) => state.returnRoute);
  const authStatus = useStore((state) => state.authStatus);
  const [lastAuthStatus, setLastAuthStatus] = useState(authStatus);

  useEffect(() => {
    console.log(
      "BlueskyProvider - authStatus:",
      authStatus,
      "lastAuthStatus:",
      lastAuthStatus,
      "returnRoute:",
      returnRoute,
    );
  }, [authStatus, lastAuthStatus, returnRoute]);

  const [lastLink, setLastLink] = useState<string | null>(null);
  const url = useLinkingURL();

  useEffect(() => {
    if (url !== lastLink && url) {
      setLastLink(url);
      // /app-return is the browser handoff page; its OAuth state belongs to
      // the app it hands off to, so this page must not consume the code.
      if (url.includes("?") && !url.includes("/app-return/")) {
        const params = new URLSearchParams(url.split("?")[1]);
        if (params.has("error") || params.has("code")) {
          oauthCallback(url);
        }
      }
    }
  }, [url, lastLink, oauthCallback]);

  // Desktop's login runs in the system browser, so its callback never appears
  // in this window's URL. The main process delivers it over IPC instead.
  useEffect(() => {
    return getDesktopAuth()?.onCallback((callbackURL) => {
      oauthCallback(callbackURL);
    });
  }, [oauthCallback]);

  // handle navigation after successful login
  useEffect(() => {
    if (
      lastAuthStatus !== "loggedIn" &&
      authStatus === "loggedIn" &&
      returnRoute &&
      Platform.OS === "web"
    ) {
      console.log(
        "Login successful, navigating back to returnRoute:",
        returnRoute,
      );
      closeLoginModal();
      console.log("BlueskyProvider is redirecting NOW to", returnRoute);
      navigateToRoute(navigation, returnRoute);
      setReturnRoute(null);
      setLastAuthStatus(authStatus);
    } else if (authStatus !== lastAuthStatus) {
      console.log("Auth status changed:", lastAuthStatus, "->", authStatus);
      setLastAuthStatus(authStatus);
    }
  }, [
    authStatus,
    lastAuthStatus,
    returnRoute,
    closeLoginModal,
    navigation,
    setReturnRoute,
  ]);

  useEffect(() => {
    if (oauthSession && !userProfile) {
      getProfile(oauthSession.did);
    }
  }, [oauthSession, userProfile]);

  return <>{children}</>;
}
