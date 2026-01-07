import { useNavigation } from "@react-navigation/native";
import { storage } from "@streamplace/components";
import { useURL } from "expo-linking";
import { useEffect, useState } from "react";
import { useStore } from "store";
import { useIsReady, useOAuthSession, useUserProfile } from "store/hooks";
import { navigateToRoute } from "utils/navigation";

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
    storage.getItem("returnRoute").then((stored) => {
      if (stored) {
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
  const url = useURL();

  useEffect(() => {
    if (url !== lastLink && url) {
      setLastLink(url);
      if (url.includes("?")) {
        const params = new URLSearchParams(url.split("?")[1]);
        if (params.has("error") || params.has("code")) {
          oauthCallback(url);
        }
      }
    }
  }, [url, lastLink, oauthCallback]);

  // handle navigation after successful login
  useEffect(() => {
    if (
      lastAuthStatus !== "loggedIn" &&
      authStatus === "loggedIn" &&
      returnRoute
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
