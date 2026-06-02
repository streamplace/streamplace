import React, { useContext, useRef } from "react";
import { makeVideoStore } from "../video-store";
import { VideoContext } from "../video-store/context";
import { useVideoFetch } from "./fetch";

export function VideoProvider({
  children,
  aturi,
  ignoreOuterContext = false,
}: {
  children: React.ReactNode;
  aturi: string;
  ignoreOuterContext?: boolean;
}) {
  const context = useContext(VideoContext);
  const store = useRef(makeVideoStore({ aturi })).current;
  if (context) {
    // this is ok, there's use cases for having one in another
    // like having a player component that's independently usable
    // but can also be embedded within an entire video page
    if (!ignoreOuterContext) {
      return <>{children}</>;
    }
  }
  (window as any).videoStore = store;
  return (
    <VideoContext.Provider value={{ store: store }}>
      <VideoPoller aturi={aturi}>{children}</VideoPoller>
    </VideoContext.Provider>
  );
}

export function VideoFetcher({ aturi }: { aturi: string }) {
  useVideoFetch(aturi);
  return <></>;
}

export function VideoPoller({
  children,
  aturi,
}: {
  children: React.ReactNode;
  aturi: string;
}) {
  return (
    <>
      <VideoFetcher aturi={aturi} />
      {children}
    </>
  );
}
