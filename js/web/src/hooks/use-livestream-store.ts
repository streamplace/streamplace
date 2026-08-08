// Hook that creates a LivestreamStore for a given user and manages
// the WebSocket connection. The store is recreated when `user` changes,
// preventing state from leaking between users on client-side navigation.
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  connectLivestreamWebsocket,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { useEffect, useMemo, useState } from "react";

export function useLivestreamStore(user: string): {
  store: LivestreamStore | null;
  ready: boolean;
} {
  // Recreate the store when user changes so stale state from the
  // previous user (chat, profile, viewers, segment, pinned message,
  // livestream record) is cleared immediately.
  const store = useMemo(() => makeLivestreamStore(), [user]);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    setReady(true);
  }, [store]);

  useEffect(() => {
    const wsUrl = `${getStreamplaceUrl()}/api/websocket/${user}`;
    const { disconnect } = connectLivestreamWebsocket(store, wsUrl, {
      reconnectDelayMs: 3000,
    });
    return disconnect;
  }, [store, user]);

  return { store: ready ? store : null, ready };
}
