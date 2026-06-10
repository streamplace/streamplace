import { getStreamplaceUrl } from "@/lib/streamplace-url";
import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { useStore } from "zustand";

export const Route = createFileRoute("/embed/danmu-obs/$user")({
  component: EmbedDanmuObs,
});

interface DanmuMessage {
  id: string;
  text: string;
  color: string;
  lane: number;
  createdAt: number;
}

function EmbedDanmuObs() {
  const { user } = Route.useParams();
  const search = Route.useSearch() as Record<string, string>;

  const opacity = parseInt(search.opacity ?? "80", 10) / 100;
  const speed = parseFloat(search.speed ?? "1");
  const laneCount = parseInt(search.lanes ?? "12", 10);
  const maxMessages = parseInt(search.max ?? "50", 10);

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
    return <div className="w-screen h-screen bg-transparent" />;
  }

  return (
    <DanmuBody
      store={store.current}
      opacity={opacity}
      speed={speed}
      laneCount={laneCount}
      maxMessages={maxMessages}
    />
  );
}

function getRgbColor(
  color?: { red: number; green: number; blue: number } | null,
): string {
  if (!color) return "#bd6e86";
  return `rgb(${color.red}, ${color.green}, ${color.blue})`;
}

function DanmuBody({
  store,
  opacity,
  speed,
  laneCount,
  maxMessages,
}: {
  store: LivestreamStore;
  opacity: number;
  speed: number;
  laneCount: number;
  maxMessages: number;
}) {
  const chat = useStore(store, (s) => s.chat);
  const prevCountRef = useRef(0);
  const [messages, setMessages] = useState<DanmuMessage[]>([]);
  const laneUsageRef = useRef<number[]>(new Array(laneCount).fill(0));
  const idRef = useRef(0);

  const assignLane = useCallback((): number => {
    const usage = laneUsageRef.current;
    let minLane = 0;
    let minCount = usage[0];
    for (let i = 1; i < usage.length; i++) {
      if (usage[i] < minCount) {
        minCount = usage[i];
        minLane = i;
      }
    }
    usage[minLane]++;
    return minLane;
  }, []);

  const releaseLane = useCallback((lane: number) => {
    if (laneUsageRef.current[lane] > 0) {
      laneUsageRef.current[lane]--;
    }
  }, []);

  useEffect(() => {
    const newMessages = chat.slice(prevCountRef.current);
    prevCountRef.current = chat.length;

    if (newMessages.length === 0) return;

    const danmuItems: DanmuMessage[] = newMessages
      .filter((msg) => !msg.deleted && msg.record.text)
      .map((msg) => ({
        id: `${++idRef.current}-${msg.uri}`,
        text: msg.record.text,
        color: getRgbColor(msg.chatProfile?.color),
        lane: assignLane(),
        createdAt: Date.now(),
      }));

    setMessages((prev) => {
      const combined = [...prev, ...danmuItems].slice(-maxMessages);
      return combined;
    });

    // Release lanes after animation completes
    const duration = (10 / speed) * 1000;
    danmuItems.forEach((item) => {
      setTimeout(() => {
        releaseLane(item.lane);
        setMessages((prev) => prev.filter((m) => m.id !== item.id));
      }, duration);
    });
  }, [chat.length, assignLane, releaseLane, speed, maxMessages]);

  const laneHeight = 100 / laneCount;

  return (
    <div
      className="w-screen h-screen bg-transparent overflow-hidden relative pointer-events-none"
      style={{ opacity }}
    >
      {messages.map((msg) => (
        <div
          key={msg.id}
          className="absolute whitespace-nowrap text-lg font-bold"
          style={{
            top: `${msg.lane * laneHeight}%`,
            color: msg.color,
            textShadow: "1px 1px 2px rgba(0,0,0,0.8)",
            animation: `danmu-scroll ${10 / speed}s linear forwards`,
            right: "-100%",
          }}
        >
          {msg.text}
        </div>
      ))}
      <style>{`
        @keyframes danmu-scroll {
          from { transform: translateX(0); }
          to { transform: translateX(calc(-100vw - 100%)); }
        }
      `}</style>
    </div>
  );
}
