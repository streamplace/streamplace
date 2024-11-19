import { useEffect } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  getProfile,
  loadOAuthClient,
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
  useEffect(() => {
    if (oauthSession && !userProfile) {
      console.log("oauthSession", oauthSession);
      dispatch(getProfile(oauthSession.did));
    }
    if (userProfile) {
      console.log(userProfile.handle);
    }
  }, [oauthSession, userProfile]);
  return <>{children}</>;
}
