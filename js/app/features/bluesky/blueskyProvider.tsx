import { useURL } from "expo-linking";
import { putIdentity } from "features/aquareum/aquareumSlice";
import useWallet from "hooks/useWallet";
import { useEffect, useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  getProfile,
  loadOAuthClient,
  oauthCallback,
  selectOAuthSession,
  selectUserProfile,
} from "./blueskySlice";

export default function BlueskyProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const dispatch = useAppDispatch();
  useEffect(() => {
    dispatch(loadOAuthClient());
  }, []);
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
        if (params.has("code") && params.has("state") && params.has("iss")) {
          dispatch(oauthCallback(url));
        }
      }
    }
  }, [url]);

  useEffect(() => {
    if (oauthSession && !userProfile) {
      dispatch(getProfile(oauthSession.did));
    }
    if (oauthSession && userProfile && wallet.address) {
      dispatch(
        putIdentity({
          handle: userProfile.handle,
          did: oauthSession?.did,
          address: wallet.address,
          signTypedData: wallet.signTypedData,
        }),
      );
    }
  }, [oauthSession, userProfile, wallet.address]);
  return <>{children}</>;
}
