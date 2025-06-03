import React, { useContext, useRef } from "react";
import { LivestreamContext, makeLivestreamStore } from "../livestream-store";
import { useLivestreamWebsocket } from "./websocket";

export function LivestreamProvider({
  children,
  src,
}: {
  children: React.ReactNode;
  src: string;
}) {
  const context = useContext(LivestreamContext);
  const store = useRef(makeLivestreamStore()).current;
  if (context) {
    // this is ok, there's use cases for having one in another
    // like having a player component that's independently usable
    // but can also be embedded within an entire livestream page
    return <>{children}</>;
  }
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
