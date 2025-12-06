import React, { useContext, useEffect, useRef } from "react";
import { useAvatars } from "../hooks";
import { StreamNotifications } from "../lib/stream-notifications";
import {
  LivestreamContext,
  makeLivestreamStore,
  useLivestreamStore,
} from "../livestream-store";
import { useLivestreamWebsocket } from "./websocket";

export function LivestreamProvider({
  children,
  src,
  onTeleport,
  ignoreOuterContext = false,
}: {
  children: React.ReactNode;
  src: string;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
  ignoreOuterContext?: boolean;
}) {
  const context = useContext(LivestreamContext);
  const store = useRef(makeLivestreamStore()).current;
  if (context) {
    // this is ok, there's use cases for having one in another
    // like having a player component that's independently usable
    // but can also be embedded within an entire livestream page
    if (!ignoreOuterContext) {
      return <>{children}</>;
    }
  }
  (window as any).livestreamStore = store;
  return (
    <LivestreamContext.Provider value={{ store: store }}>
      <LivestreamPoller src={src} onTeleport={onTeleport}>
        {children}
      </LivestreamPoller>
    </LivestreamContext.Provider>
  );
}

export function WebsocketWatcher({ src }: { src: string }) {
  useLivestreamWebsocket(src);
  return <></>;
}

export function TeleportWatcher({
  onTeleport,
}: {
  onTeleport?: (targetHandle: string, targetDID: string) => void;
}) {
  const activeTeleport = useLivestreamStore((state) => state.activeTeleport);
  const profile = useAvatars(activeTeleport ? [activeTeleport.streamer] : []);

  useEffect(() => {
    if (!activeTeleport || !profile[activeTeleport.streamer]) return;

    const startsAt = new Date(activeTeleport.startsAt);
    const now = new Date();
    const countdown = Math.max(
      0,
      Math.floor((startsAt.getTime() - now.getTime()) / 1000),
    );

    // resolve the DID to a handle for display
    const targetHandle =
      profile[activeTeleport.streamer]?.handle || activeTeleport.streamer;

    StreamNotifications.teleport({
      targetHandle: targetHandle,
      targetDID: activeTeleport.streamer,
      countdown: countdown,
      onCancel: () => {
        console.log("Teleport cancelled by user");
      },
      onAutoDismiss: () => {
        console.log("Teleport dismissed bestie!");
        if (onTeleport) {
          console.log("Calling onTeleport callback");
          onTeleport(targetHandle, activeTeleport.streamer);
        }
      },
    });
  }, [activeTeleport, profile, onTeleport]);

  return <></>;
}

export function LivestreamPoller({
  children,
  src,
  onTeleport,
}: {
  children: React.ReactNode;
  src: string;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
}) {
  // Websocket watcher is a sibling instead of a parent to avoid
  // re-rendering when the websocket does stuff
  return (
    <>
      <WebsocketWatcher src={src} />
      <TeleportWatcher onTeleport={onTeleport} />
      {children}
    </>
  );
}
