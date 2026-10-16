// WebSocket connection lifecycle for the livestream store. Framework- and
// React-free: owns the socket, message batching, and reconnect, and feeds
// inbound messages through handleWebSocketMessages. The caller owns the
// store and calls disconnect() to tear down.
//
// `WebSocket` is only referenced inside connect(), never at module load, so
// importing this module is safe in non-browser contexts (SSR, tests). The
// caller is responsible for providing a WebSocket global when connecting.
import { type LivestreamStore } from "./store";
import { handleWebSocketMessages } from "./websocket-consumer";

export type ConnectLivestreamWebsocketOptions = {
  /** Delay before reconnecting after an unexpected close/error, in ms. */
  reconnectDelayMs?: number;
  /** Coalesce window for inbound messages, in ms. 0 flushes on the next macrotask. */
  batchWindowMs?: number;
  /** Called once when the socket drops, before a reconnect is scheduled. */
  onDisconnect?: () => void;
};

export type LivestreamWebsocketHandle = {
  /** Closes the socket and cancels any pending reconnect/flush timers. */
  disconnect: () => void;
};

export function connectLivestreamWebsocket(
  store: LivestreamStore,
  url: string,
  options: ConnectLivestreamWebsocketOptions = {},
): LivestreamWebsocketHandle {
  const reconnectDelayMs = options.reconnectDelayMs ?? 3000;
  const batchWindowMs = options.batchWindowMs ?? 0;

  let ws: WebSocket | null = null;
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  let flushTimer: ReturnType<typeof setTimeout> | null = null;
  let messageBuffer: unknown[] = [];
  let currentConnectId = 0;
  let disposed = false;
  let reconnectURL = url;

  const flush = () => {
    flushTimer = null;
    const batch = messageBuffer;
    messageBuffer = [];
    store.setState((s) => handleWebSocketMessages(s, batch));
  };

  const scheduleReconnect = () => {
    if (disposed || reconnectTimeout) return;
    currentConnectId++;
    if (flushTimer) clearTimeout(flushTimer);
    flushTimer = null;
    const state = handleWebSocketMessages(store.getState(), messageBuffer);
    messageBuffer = [];
    // Keep a prior hint only if this connection failed before receiving state.
    if (state.websocketConnected) reconnectURL = url;
    const { activeTeleportUri, activeTeleportCID } = state;
    if (activeTeleportUri && activeTeleportCID) {
      const resumeURL = new URL(reconnectURL);
      resumeURL.searchParams.set("teleportUri", activeTeleportUri);
      resumeURL.searchParams.set("teleportCid", activeTeleportCID);
      reconnectURL = resumeURL.toString();
    }
    // A cancellation may be missed while offline. Only a fresh snapshot can
    // restore the teleport, including one that started while disconnected.
    store.setState({
      websocketConnected: false,
      activeTeleport: null,
      activeTeleportUri: null,
      activeTeleportCID: null,
    });
    options.onDisconnect?.();
    reconnectTimeout = setTimeout(connect, reconnectDelayMs);
  };

  const connect = () => {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout);
      reconnectTimeout = null;
    }

    const connectId = ++currentConnectId;

    ws = new WebSocket(reconnectURL);

    ws.onopen = () => {};

    ws.onmessage = (event) => {
      if (connectId !== currentConnectId) return;
      try {
        const messages = JSON.parse(event.data);
        const list = Array.isArray(messages) ? messages : [messages];
        messageBuffer.push(...list);
        if (!flushTimer) {
          flushTimer = setTimeout(flush, batchWindowMs);
        }
      } catch {
        // Ignore malformed frames; the server heartbeat keeps the socket alive.
      }
    };

    ws.onclose = () => {
      if (connectId !== currentConnectId) return;
      scheduleReconnect();
    };

    ws.onerror = () => {
      if (connectId !== currentConnectId) return;
      ws?.close();
      scheduleReconnect();
    };
  };

  connect();

  return {
    disconnect: () => {
      disposed = true;
      currentConnectId++;
      messageBuffer = [];
      if (flushTimer) clearTimeout(flushTimer);
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
      store.setState({
        websocketConnected: false,
        activeTeleport: null,
        activeTeleportUri: null,
        activeTeleportCID: null,
      });
      ws?.close();
    },
  };
}
