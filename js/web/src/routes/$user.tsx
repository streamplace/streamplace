import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { createFileRoute, Link } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { ChatSidebar } from "../components/stream/chat-sidebar";
import { StreamInfo } from "../components/stream/stream-info";
import { VideoSection } from "../components/stream/video-section";
import { useLivenessState } from "../hooks/use-liveness-state";
import { getStreamplaceUrl } from "../lib/streamplace-url";

export const Route = createFileRoute("/$user")({
  component: StreamPage,
});

function StreamPage() {
  const { user } = Route.useParams();
  const store = useRef<LivestreamStore | null>(null);
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    const s = makeLivestreamStore();
    store.current = s;
    setInitialized(true);
  }, []);

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

      ws.onmessage = (event) => {
        if (connectId !== currentConnectId) return;
        try {
          const messages = JSON.parse(event.data);
          const list = Array.isArray(messages) ? messages : [messages];
          store.current?.setState((s) => handleWebSocketMessages(s, list));
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
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
      ws?.close();
    };
  }, [user]);

  if (!initialized || !store.current) {
    return (
      <div className="max-w-6xl mx-auto px-6 py-12">
        <div className="animate-pulse">
          <div className="h-8 bg-[var(--color-bg-elevated)] rounded w-48 mb-4" />
          <div className="aspect-video bg-[var(--color-bg-elevated)] rounded" />
        </div>
      </div>
    );
  }

  return <StreamBody store={store.current} user={user} />;
}

function StreamBody({ store, user }: { store: LivestreamStore; user: string }) {
  const liveness = useLivenessState(store);
  const [chatOpen, setChatOpen] = useState(() => {
    if (typeof localStorage === "undefined") return true;
    return localStorage.getItem("streamplace:chat-open") !== "false";
  });

  const toggleChat = useCallback(() => {
    setChatOpen((prev) => {
      const next = !prev;
      localStorage.setItem("streamplace:chat-open", String(next));
      return next;
    });
  }, []);

  if (liveness === "offline") {
    return <OfflinePage user={user} />;
  }

  return (
    <div className="flex flex-col gap-3 h-full">
      <div
        className={`flex-1 flex min-h-0 gap-4 transition-[margin] duration-300 ease-in-out ${chatOpen ? "mr-[360px]" : "mr-0"}`}
      >
        <div className="flex-1 min-w-0 overflow-y-auto">
          <VideoSection store={store} user={user} liveness={liveness} />
          <StreamInfo
            store={store}
            user={user}
            liveness={liveness}
            chatOpen={chatOpen}
            onToggleChat={toggleChat}
          />
        </div>
      </div>

      <div
        className={`fixed top-12 bottom-0 right-0 w-[360px] max-w-90 flex flex-col overflow-hidden transition-transform duration-300 ease-in-out z-20 ${
          chatOpen ? "translate-x-0" : "translate-x-full"
        }`}
      >
        <ChatSidebar store={store} onClose={toggleChat} />
      </div>
    </div>
  );
}

function OfflinePage({ user }: { user: string }) {
  return (
    <div className="max-w-2xl mx-auto px-6 py-20 text-center">
      <div className="inline-flex items-center justify-center w-14 h-14 rounded-full bg-[var(--color-bg-elevated)] border border-[var(--color-border)] mb-6">
        <svg
          className="w-5 h-5 text-[var(--color-fg-subtle)]"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={1.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M5.636 5.636a9 9 0 1 0 12.728 12.728A9 9 0 0 0 5.636 5.636Z"
          />
        </svg>
      </div>
      <h1 className="text-2xl font-semibold">Stream is offline</h1>
      <p className="text-sm text-[var(--color-fg-muted)] mt-2">
        <span className="font-mono">{user}</span> is not currently streaming.
        Check back later.
      </p>
      <div className="mt-8 flex items-center justify-center gap-3">
        <Link
          to="/"
          className="h-9 inline-flex items-center px-4 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-[var(--color-accent-fg)] text-sm font-medium"
        >
          Back to home
        </Link>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="h-9 px-4 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-sm"
        >
          Refresh
        </button>
      </div>
    </div>
  );
}
