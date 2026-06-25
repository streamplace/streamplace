import { ChatSidebar } from "@/components/stream/chat-sidebar";
import { StreamInfo } from "@/components/stream/stream-info";
import { VideoSection } from "@/components/stream/video-section";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { useFullscreen } from "@/contexts/fullscreen-context";
import { useLivenessState } from "@/hooks/use-liveness-state";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { ChevronUp, ExternalLink } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";

export const Route = createFileRoute("/$user/")({
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
      <div className="mx-auto max-w-6xl px-6 py-12">
        <div className="animate-pulse">
          <div className="mb-4 h-8 w-48 rounded bg-(--color-bg-elevated)" />
          <div className="aspect-video rounded bg-(--color-bg-elevated)" />
        </div>
      </div>
    );
  }

  return <StreamBody store={store.current} user={user} />;
}

function StreamBody({ store, user }: { store: LivestreamStore; user: string }) {
  const liveness = useLivenessState(store);
  const { theatre } = useFullscreen();
  // Offline streams default to a closed chat; the chat has nothing
  // to show, and the shorter offline player should take the page
  // width. Live streams honor the user's saved preference.
  const isOffline = liveness === "offline" || liveness === "never-live";
  const [chatOpen, setChatOpen] = useState(() => {
    if (isOffline) return false;
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

  // Auto-close the chat when a live stream goes offline. The user
  // can still open it manually to see the backlog; this just gets
  // the page back to a sensible default. (The reverse case; a
  // closed chat reopening when the stream goes live; doesn't
  // auto-open, the user has the toggle for that.)
  useEffect(() => {
    if (isOffline && chatOpen) {
      setChatOpen(false);
    }
  }, [isOffline, chatOpen]);

  return (
    <div className="flex h-full flex-col">
      {/* Sidebar layout (wide viewport) */}
      <div className="wide:flex wide:h-full wide:flex-col wide:gap-3 hidden">
        <div
          className={`z-0 flex min-h-0 flex-1 gap-4 transition-[margin] duration-300 ease-in-out ${chatOpen ? "wide:mr-90" : ""}`}
        >
          <div className="min-w-0 flex-1 overflow-y-auto">
            <VideoSection store={store} user={user} liveness={liveness} />
            {!theatre && (
              <StreamInfo
                store={store}
                user={user}
                liveness={liveness}
                chatOpen={chatOpen}
                onToggleChat={toggleChat}
              />
            )}
          </div>
        </div>

        <div
          className={`fixed top-12 right-0 bottom-0 z-20 flex w-90 flex-col overflow-hidden transition-transform duration-300 ease-in-out ${
            chatOpen ? "translate-x-0" : "translate-x-full"
          }`}
        >
          <ChatSidebar store={store} onClose={toggleChat} />
        </div>
      </div>

      {/* Stacked layout (portrait/tall) */}
      <div className="wide:hidden flex min-h-0 flex-1 flex-col">
        <VideoSection store={store} user={user} liveness={liveness} />

        <MobileStreamBar store={store} user={user} />

        <div className="flex min-h-0 flex-1 flex-col border-t">
          <ChatSidebar store={store} />
        </div>
      </div>
    </div>
  );
}

function MobileStreamBar({
  store,
  user,
}: {
  store: LivestreamStore;
  user: string;
}) {
  const { t } = useTranslation("common");
  const state = useStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
      viewers: s.viewers,
    })),
  );

  const author = state.livestream?.author;
  const record = state.livestream?.record;
  const title = record?.title || user;

  return (
    <div className="flex items-center gap-2 border-b px-3 py-2">
      <img
        src={author?.avatar ?? undefined}
        alt=""
        className="h-7 w-7 shrink-0 rounded-full bg-(--color-bg-elevated)"
        onError={(e) => {
          (e.currentTarget as HTMLImageElement).style.display = "none";
        }}
      />

      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{title}</div>
        <div className="truncate text-xs text-(--color-fg-muted)">
          {author?.displayName || author?.handle || user}
          {state.viewers != null && (
            <> &middot; {t("watching-count", { count: state.viewers })}</>
          )}
        </div>
      </div>

      <Sheet>
        <SheetTrigger
          render={
            <button
              type="button"
              className="rounded p-1.5 text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
              aria-label={t("stream-info")}
            >
              <ChevronUp className="h-4 w-4" />
            </button>
          }
        />
        <SheetContent side="bottom" className="rounded-t-xl">
          <SheetHeader>
            <SheetTitle>{title}</SheetTitle>
          </SheetHeader>
          <div className="px-4 pb-6">
            <div className="flex items-center gap-3">
              <img
                src={author?.avatar ?? undefined}
                alt=""
                className="h-10 w-10 shrink-0 rounded-full bg-(--color-bg-elevated)"
                onError={(e) => {
                  (e.currentTarget as HTMLImageElement).style.display = "none";
                }}
              />
              <div className="min-w-0 flex-1">
                <div className="font-medium">
                  {author?.displayName || author?.handle || user}
                </div>
                {author?.handle && (
                  <div className="text-sm text-(--color-fg-muted)">
                    @{author.handle}
                  </div>
                )}
              </div>
              <a
                href={`https://bsky.app/profile/${author?.handle || ""}`}
                target="_blank"
                rel="noopener noreferrer"
                className="rounded p-1.5 text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
              >
                <ExternalLink className="h-4 w-4" />
              </a>
            </div>
            {record?.description ? (
              <p className="mt-3 text-sm whitespace-pre-wrap text-(--color-fg)">
                {record.description as string}
              </p>
            ) : null}
          </div>
        </SheetContent>
      </Sheet>

      <a
        href={`/chat-popout/${encodeURIComponent(author?.handle || "")}`}
        target="_blank"
        rel="noopener noreferrer"
        className="rounded p-1.5 text-(--color-fg-muted) transition-colors hover:bg-(--color-bg-overlay) hover:text-(--color-fg)"
        title={t("chat-pop-out")}
      >
        <ExternalLink className="h-4 w-4" />
      </a>
    </div>
  );
}
