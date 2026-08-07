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
import { createFileRoute, Link } from "@tanstack/react-router";
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

function OfflinePage({ user }: { user: string }) {
  const { t } = useTranslation("common");
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 text-center">
      <div className="mb-6 inline-flex h-14 w-14 items-center justify-center rounded-full border border-(--color-border) bg-(--color-bg-elevated)">
        <svg
          className="h-5 w-5 text-(--color-fg-subtle)"
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
      <h1 className="font-display text-2xl font-semibold">
        {t("stream-is-offline-title")}
      </h1>
      <p className="mt-2 text-sm text-(--color-fg-muted)">
        {t("user-not-streaming-check-back", { user })}
      </p>
      <div className="mt-8 flex items-center justify-center gap-3">
        <Link
          to="/"
          className="inline-flex h-9 items-center rounded-md bg-(--color-accent) px-4 text-sm font-medium text-(--color-accent-fg) hover:bg-(--color-accent-hover)"
        >
          {t("back-to-home")}
        </Link>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="h-9 rounded-md border border-(--color-border) px-4 text-sm hover:border-(--color-border-strong)"
        >
          {t("refresh")}
        </button>
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
            {(record as any)?.description ? (
              <p className="mt-3 text-sm whitespace-pre-wrap text-(--color-fg)">
                {(record as any).description as string}
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
