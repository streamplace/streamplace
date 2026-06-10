import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { useStore } from "zustand";

export const Route = createFileRoute("/embed/info-widget/$user")({
  component: EmbedInfoWidget,
});

function EmbedInfoWidget() {
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
      <div className="w-screen h-screen bg-transparent flex items-center justify-center">
        <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
      </div>
    );
  }

  return <InfoWidgetBody store={store.current} />;
}

function InfoWidgetBody({ store }: { store: LivestreamStore }) {
  const state = useStore(store, (s) => ({
    livestream: s.livestream,
    segment: s.segment,
    viewers: s.viewers,
    websocketConnected: s.websocketConnected,
  }));

  const videoTrack = state.segment?.video?.[0];
  const width = videoTrack?.width ?? 0;
  const height = videoTrack?.height ?? 0;

  const bitrate =
    state.segment?.size && state.segment?.duration
      ? (
          (state.segment.size * 8) /
          (state.segment.duration / 1_000_000_000) /
          1000
        ).toFixed(0)
      : "0";

  const fps = videoTrack?.framerate
    ? Math.round(videoTrack.framerate.num / videoTrack.framerate.den)
    : null;

  return (
    <div className="w-screen h-screen bg-transparent flex items-start justify-end p-4">
      <div className="bg-black/70 backdrop-blur-sm rounded-lg p-4 text-white text-sm min-w-[200px] font-mono space-y-2">
        <div className="flex items-center gap-2">
          <div
            className={`w-2 h-2 rounded-full ${state.websocketConnected ? "bg-green-400" : "bg-red-400"}`}
          />
          <span className="text-xs uppercase tracking-wider opacity-70">
            {state.websocketConnected ? "connected" : "disconnected"}
          </span>
        </div>

        {state.viewers !== null && (
          <div className="flex justify-between">
            <span className="opacity-70">Viewers</span>
            <span>{state.viewers}</span>
          </div>
        )}

        {width > 0 && height > 0 && (
          <div className="flex justify-between">
            <span className="opacity-70">Resolution</span>
            <span>
              {width}x{height}
            </span>
          </div>
        )}

        {fps !== null && (
          <div className="flex justify-between">
            <span className="opacity-70">FPS</span>
            <span>{fps}</span>
          </div>
        )}

        <div className="flex justify-between">
          <span className="opacity-70">Bitrate</span>
          <span>{bitrate} kbps</span>
        </div>
      </div>
    </div>
  );
}
