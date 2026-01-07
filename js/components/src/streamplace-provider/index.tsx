import { SessionManager } from "@atproto/api/dist/session-manager";
import { useEffect, useRef } from "react";
import { useGetChatProfile } from "../streamplace-store";
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
      <ChatProfileCreator oauthSession={oauthSession}>
        <Poller>{children}</Poller>
      </ChatProfileCreator>
    </StreamplaceContext.Provider>
  );
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
