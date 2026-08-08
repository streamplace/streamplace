import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { useEffect, useMemo, type ReactNode } from "react";

interface LivestreamProviderProps {
  /** The DID or handle of the user to connect to. */
  user: string;
  /** Render prop that receives the initialized store. */
  children: (store: LivestreamStore) => ReactNode;
}

/**
 * Creates a LivestreamStore, connects a WebSocket for the given user,
 * and passes the store to children via a render prop. The WebSocket
 * reconnects automatically on close/error.
 */
export function LivestreamProvider({
  user,
  children,
}: LivestreamProviderProps) {
  const store = useMemo(() => makeLivestreamStore(), [user]);

  // WebSocket connection
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

  return <>{children(store)}</>;
}
