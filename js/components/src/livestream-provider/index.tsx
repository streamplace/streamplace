import { makeLivestreamStore } from "@streamplace/core";
import React, { useContext, useEffect, useRef } from "react";
import { useAvatars } from "../hooks";
import { deleteTeleport } from "../lib/slash-commands/teleport";
import { StreamNotifications } from "../lib/stream-notifications";
import { LivestreamContext } from "../livestream-store/context";
import { useUnpinChatMessage } from "../livestream-store/hooks";
import {
  getStoreFromContext,
  useLivestreamStore,
  usePinnedComment,
} from "../livestream-store/use-store";
import { useDID, usePDSAgent } from "../streamplace-store";
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
  const activeTeleportUri = useLivestreamStore(
    (state) => state.activeTeleportUri,
  );
  const profile = useAvatars(activeTeleport ? [activeTeleport.streamer] : []);
  const livestreamProfile = useLivestreamStore((state) => state.profile);
  const pdsAgent = usePDSAgent();
  const userDID = useDID();
  const prevActiveTeleportRef = useRef(activeTeleport);

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

    // check if the current user is the streamer of the current livestream
    const canCancel = livestreamProfile?.did === userDID;

    StreamNotifications.teleport({
      targetHandle: targetHandle,
      targetDID: activeTeleport.streamer,
      countdown: countdown,
      canCancel: canCancel,
      onDismiss: async (reason) => {
        console.log(
          "🔍 StreamNotifications.onDismiss called with reason:",
          reason,
        );
        if (reason === "user" && activeTeleportUri && pdsAgent && userDID) {
          try {
            await deleteTeleport(pdsAgent, userDID, activeTeleportUri);
          } catch (err) {
            console.error("Failed to delete teleport:", err);
          }
        }
        if (reason === "auto" && onTeleport) {
          console.log(
            "🔍 Calling onTeleport with:",
            targetHandle,
            activeTeleport.streamer,
          );
          onTeleport(targetHandle, activeTeleport.streamer);
        } else if (reason === "auto" && !onTeleport) {
          console.log("🔍 onTeleport is not defined!");
        } else if (reason === "auto") {
          console.log(
            "🔍 Reason is auto but teleport function not called for unknown reason",
          );
        }
      },
    });
  }, [
    activeTeleport,
    activeTeleportUri,
    profile,
    onTeleport,
    pdsAgent,
    userDID,
  ]);

  useEffect(() => {
    if (
      prevActiveTeleportRef.current &&
      !activeTeleport &&
      !activeTeleportUri
    ) {
      StreamNotifications.teleportCancelled();
    }
    prevActiveTeleportRef.current = activeTeleport;
  }, [activeTeleport, activeTeleportUri]);

  return <></>;
}

export function PinnedCommentWatcher() {
  const pinnedComment = usePinnedComment();
  const streamerDID = useLivestreamStore((state) => state.profile?.did);
  const unpinChatMessage = useUnpinChatMessage();
  const store = getStoreFromContext();
  const prevPinnedRef = useRef<string | null>(null);

  useEffect(() => {
    return () => {
      StreamNotifications.pinnedCommentDismiss();
    };
  }, []);

  // Show/hide notification when pinned comment changes
  useEffect(() => {
    const currentUri = pinnedComment?.uri ?? null;
    if (currentUri === prevPinnedRef.current) return;
    prevPinnedRef.current = currentUri;

    if (pinnedComment) {
      StreamNotifications.pinnedComment({
        pinnedComment,
        onDismiss: () => {
          store.setState({ pinnedComment: null });
        },
        onUnpin: () => {
          if (!streamerDID) return;
          unpinChatMessage(pinnedComment.uri, streamerDID).catch((e) => {
            console.error("Failed to unpin:", e);
          });
        },
      });
    } else {
      StreamNotifications.pinnedCommentDismiss();
    }
  }, [pinnedComment, streamerDID, unpinChatMessage]);

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
  return (
    <>
      <WebsocketWatcher src={src} />
      <TeleportWatcher onTeleport={onTeleport} />
      <PinnedCommentWatcher />
      {children}
    </>
  );
}
