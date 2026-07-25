import { SessionManager } from "@atproto/api/dist/session-manager";
import { useEffect, useRef } from "react";
import { ProfileCacheProvider } from "../context/profile-cache";
import { useDocumentTitle } from "../hooks";
import {
  useBrandingAutoFetch,
  useFetchBroadcasterDID,
  useFetchEnvConfig,
  useGetChatProfile,
} from "../streamplace-store";
import { makeStreamplaceStore } from "../streamplace-store/streamplace-store";
import { StreamplaceContext } from "./context";
import Poller from "./poller";

export function StreamplaceProvider({
  children,
  url,
  oauthSession,
}: {
  children: React.ReactNode;
  url: string;
  oauthSession?: SessionManager | null;
}) {
  // todo: handle url changes?
  const store = useRef(makeStreamplaceStore({ url })).current;

  useEffect(() => {
    store.setState({ url });
  }, [url]);

  useEffect(() => {
    store.setState({ oauthSession });
  }, [oauthSession]);

  return (
    <StreamplaceContext.Provider value={{ store: store }}>
      <ProfileCacheProvider>
        <BrandingFetcher>
          <ChatProfileCreator oauthSession={oauthSession}>
            <Poller>{children}</Poller>
          </ChatProfileCreator>
        </BrandingFetcher>
      </ProfileCacheProvider>
    </StreamplaceContext.Provider>
  );
}

export function BrandingFetcher({ children }: { children: React.ReactNode }) {
  const fetchBroadcasterDID = useFetchBroadcasterDID();
  const fetchEnvConfig = useFetchEnvConfig();
  useBrandingAutoFetch();
  useDocumentTitle();

  useEffect(() => {
    fetchBroadcasterDID();
    fetchEnvConfig();
  }, [fetchBroadcasterDID, fetchEnvConfig]);

  return <>{children}</>;
}

export function ChatProfileCreator({
  oauthSession,
  children,
}: {
  oauthSession?: SessionManager | null;
  children: React.ReactNode;
}) {
  const getChatProfile = useGetChatProfile();
  useEffect(() => {
    if (oauthSession) {
      getChatProfile();
    }
  }, [oauthSession]);
  return <>{children}</>;
}
