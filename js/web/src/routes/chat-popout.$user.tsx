// Standalone chat popout window. Minimal chrome — just the chat panel
// and input, no player, no sidebar, no header.
//
// Query parameters (useful for OBS browser sources):
//   reverse=true     Show newest messages at top
//   reverse=false    Show newest messages at bottom (default)
//   hideAfter=N      Hide the chat after N seconds
//   hideChatBox=true Hide the chat input (read-only)
//   hidePinnedComments=true  Hide pinned comment notifications
import { ChatInput } from "@/components/stream/chat-input";
import { ChatPanel } from "@/components/stream/chat-panel";
import { StreamNotifications } from "@/components/stream/stream-notifications";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

export const Route = createFileRoute("/chat-popout/$user")({
  validateSearch: (
    search: Record<string, unknown>,
  ): {
    reverse?: boolean;
    hideAfter?: number;
    hideChatBox?: boolean;
    hidePinnedComments?: boolean;
  } => {
    const toBool = (v: unknown): boolean | undefined => {
      if (v === undefined || v === null) return undefined;
      if (typeof v === "boolean") return v;
      return String(v).toLowerCase() === "true";
    };
    const toNum = (v: unknown): number | undefined => {
      if (typeof v === "number" && !isNaN(v)) return v;
      if (typeof v === "string" && !isNaN(Number(v))) return Number(v);
      return undefined;
    };
    const result: {
      reverse?: boolean;
      hideAfter?: number;
      hideChatBox?: boolean;
      hidePinnedComments?: boolean;
    } = {};
    const reverse = toBool(search.reverse);
    if (reverse !== undefined) result.reverse = reverse;
    const hideAfter = toNum(search.hideAfter);
    if (hideAfter !== undefined) result.hideAfter = hideAfter;
    const hideChatBox = toBool(search.hideChatBox);
    if (hideChatBox !== undefined) result.hideChatBox = hideChatBox;
    const hidePinnedComments = toBool(search.hidePinnedComments);
    if (hidePinnedComments !== undefined)
      result.hidePinnedComments = hidePinnedComments;
    return result;
  },
  component: ChatPopoutPage,
});

function ChatPopoutPage() {
  const { user } = Route.useParams();
  const { reverse, hideAfter, hideChatBox, hidePinnedComments } =
    Route.useSearch();
  const reverseVal = reverse ?? false;
  const hideChatBoxVal = hideChatBox ?? false;
  const hidePinnedCommentsVal = hidePinnedComments ?? false;
  const store = useRef<LivestreamStore | null>(null);
  const [initialized, setInitialized] = useState(false);
  const [hidden, setHidden] = useState(false);

  useEffect(() => {
    const s = makeLivestreamStore();
    store.current = s;
    setInitialized(true);
  }, []);

  // hideAfter timer
  useEffect(() => {
    if (!hideAfter || hideAfter <= 0) return;
    const timer = setTimeout(() => setHidden(true), hideAfter * 1000);
    return () => clearTimeout(timer);
  }, [hideAfter]);

  useEffect(() => {
    if (!store.current) return;

    const wsUrl = `${getStreamplaceUrl()}/api/websocket/${user}`;

    let ws: WebSocket | null = null;
    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
    let currentConnectId = 0;
    let mounted = true;

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
      let flushTimer: ReturnType<typeof setTimeout> | null = null;
      const flush = () => {
        flushTimer = null;
        const batch = messageBuffer;
        messageBuffer = [];
        store.current?.setState((s) => handleWebSocketMessages(s, batch));
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
        store.current?.setState((s) => ({ ...s, websocketConnected: false }));
        scheduleReconnect();
      };

      ws.onerror = () => {
        if (connectId !== currentConnectId) return;
        store.current?.setState((s) => ({ ...s, websocketConnected: false }));
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
  }, [user]);

  if (!initialized || !store.current) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-[var(--color-border)] border-t-[var(--color-accent)]" />
      </div>
    );
  }

  return (
    <div
      className="flex h-screen flex-col bg-[var(--color-background)]"
      style={
        hidden ? { opacity: 0, pointerEvents: "none" as const } : undefined
      }
    >
      {!hidePinnedCommentsVal && <StreamNotifications store={store.current} />}
      <div className="flex min-h-0 flex-1 flex-col">
        <ChatPanel store={store.current} reversed={reverseVal} />
      </div>
      {!hideChatBoxVal && (
        <div className="border-t p-2">
          <ChatInput store={store.current} />
        </div>
      )}
    </div>
  );
}
