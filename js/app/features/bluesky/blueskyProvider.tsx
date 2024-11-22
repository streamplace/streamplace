import { useEffect } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  getProfile,
  loadOAuthClient,
  selectOAuthSession,
  selectUserProfile,
} from "./blueskySlice";
import { putIdentity } from "features/aquareum/aquareumSlice";
import useWallet from "hooks/useWallet";

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
  useEffect(() => {
    if (oauthSession && !userProfile) {
      console.log("oauthSession", oauthSession);
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
    console.log(wallet);
  }, [oauthSession, userProfile, wallet.address]);
  return <>{children}</>;
}
