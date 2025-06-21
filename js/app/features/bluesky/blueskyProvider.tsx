import { useURL } from "expo-linking";
import useWallet from "hooks/useWallet";
import { useEffect, useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  getProfile,
  loadOAuthClient,
  oauthCallback,
  selectIsReady,
  selectOAuthSession,
  selectUserProfile,
} from "./blueskySlice";

export default function BlueskyProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const dispatch = useAppDispatch();
  const isReady = useAppSelector(selectIsReady);
  useEffect(() => {
    dispatch(loadOAuthClient());
  }, []);
  useEffect(() => {
    if (!isReady) {
      const handle = setInterval(() => {
        dispatch(loadOAuthClient());
      }, 5000);
      return () => clearInterval(handle);
    }
  }, [isReady]);
  const oauthSession = useAppSelector(selectOAuthSession);
  const userProfile = useAppSelector(selectUserProfile);
  const wallet = useWallet();

  const [lastLink, setLastLink] = useState<string | null>(null);
  const url = useURL();

  useEffect(() => {
    if (url !== lastLink && url) {
      setLastLink(url);
      if (url.includes("?")) {
        const params = new URLSearchParams(url.split("?")[1]);
        if (params.has("error") || params.has("code")) {
          dispatch(oauthCallback(url));
        }
      }
    }
  }, [url, lastLink]);

  useEffect(() => {
    if (oauthSession && !userProfile) {
      dispatch(getProfile(oauthSession.did));
    }
  }, [oauthSession, userProfile, wallet.address]);
  return <>{children}</>;
}
