import React, { useRef } from "react";
import { LivestreamContext, makeLivestreamStore } from "../livestream-store";
import { useLivestreamWebsocket } from "./websocket";

export function LivestreamProvider({
  children,
  src,
}: {
  children: React.ReactNode;
  src: string;
}) {
  const store = useRef(makeLivestreamStore()).current;
  (window as any).livestreamStore = store;
  return (
    <LivestreamContext.Provider value={{ store: store }}>
      <LivestreamPoller src={src}>{children}</LivestreamPoller>
    </LivestreamContext.Provider>
  );
}

export function LivestreamPoller({
  children,
  src,
}: {
  children: React.ReactNode;
  src: string;
}) {
  useLivestreamWebsocket(src);
  return <>{children}</>;
}
