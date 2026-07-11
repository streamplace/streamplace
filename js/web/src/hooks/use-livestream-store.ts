// Hook that creates a LivestreamStore for a given user and manages
// the WebSocket connection. The store is recreated when `user` changes,
// preventing state from leaking between users on client-side navigation.
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  handleWebSocketMessages,
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

    let ws: WebSocket | null = null;
    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
    let currentConnectId = 0;
    let mounted = true;
    let flushTimer: ReturnType<typeof setTimeout> | null = null;

    const scheduleReconnect = () => {
      if (!mounted || reconnectTimeout) return;
      reconnectTimeout = setTimeout(connect, 3000);
    };

    const connect = () => {
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
        reconnectTimeout = null;
      }

      const connectId = ++currentConnectId;

      ws = new WebSocket(wsUrl);

      ws.onopen = () => {};

      let messageBuffer: any[] = [];
      const flush = () => {
        flushTimer = null;
        const batch = messageBuffer;
        messageBuffer = [];
        store.setState((s) => handleWebSocketMessages(s, batch));
      };

      ws.onmessage = (event) => {
        if (connectId !== currentConnectId) return;
        try {
          const messages = JSON.parse(event.data);
          const list = Array.isArray(messages) ? messages : [messages];
          messageBuffer.push(...list);
          if (!flushTimer) {
            flushTimer = setTimeout(flush, 0);
          }
        } catch {}
      };

      ws.onclose = () => {
        if (connectId !== currentConnectId) return;
        store.setState((s) => ({ ...s, websocketConnected: false }));
        scheduleReconnect();
      };

      ws.onerror = () => {
        if (connectId !== currentConnectId) return;
        store.setState((s) => ({ ...s, websocketConnected: false }));
        ws?.close();
        scheduleReconnect();
      };
    };

    connect();

    return () => {
      mounted = false;
      if (flushTimer) clearTimeout(flushTimer);
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
      ws?.close();
    };
  }, [store, user]);

  return { store: ready ? store : null, ready };
}
