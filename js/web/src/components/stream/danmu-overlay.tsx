import type { LivestreamStore } from "@streamplace/core";
import { memo, useCallback, useEffect, useRef, useState } from "react";
import type { ChatMessageViewHydrated } from "streamplace";
import { useStore } from "zustand";
import { useDanmuLanes } from "./use-danmu-lanes";

const MIN_DURATION = 6000;
const MAX_DURATION = 12000;
const DEFAULT_LANE_COUNT = 12;
const DEFAULT_OPACITY = 80;
const DEFAULT_SPEED = 1;
const DEFAULT_MAX_MESSAGES = 50;
const FONT_SIZE_PERCENTAGE = 0.7;
const MAX_PROCESSED_MESSAGES = 10;

// px from top of video where danmu won't appear (avoid overlapping with title)
const TOP_GAP = 20;
// px from bottom of video (avoid overlapping with controls)
const BOTTOM_GAP = 20;

function mapRange(
  num: number,
  inMin: number,
  inMax: number,
  outMin: number,
  outMax: number,
) {
  return ((num - inMin) * (outMax - outMin)) / (inMax - inMin) + outMin;
}

function baseDuration(message: { record: { text: string } }) {
  const len = message.record.text.length;
  return Math.min(
    MAX_DURATION,
    Math.max(
      MIN_DURATION,
      mapRange(Math.log(len) * 8, 1, 16, MIN_DURATION, MAX_DURATION),
    ),
  );
}

function brightenColor(
  color: { red: number; green: number; blue: number } = {
    red: 123,
    green: 123,
    blue: 123,
  },
) {
  const red = mapRange(color.red, 0, 255, 100, 230);
  const green = mapRange(color.green, 0, 255, 100, 230);
  const blue = mapRange(color.blue, 0, 255, 100, 230);
  return `rgb(${Math.round(red)}, ${Math.round(green)}, ${Math.round(blue)})`;
}

interface DanmuOverlayProps {
  store: LivestreamStore;
  enabled?: boolean;
  opacity?: number;
  speed?: number;
  laneCount?: number;
  maxMessages?: number;
}

interface ActiveDanmuMessage {
  message: ChatMessageViewHydrated;
  lane: number;
  duration: number;
}

export function DanmuOverlay({
  store,
  enabled = true,
  opacity = DEFAULT_OPACITY,
  speed = DEFAULT_SPEED,
  laneCount = DEFAULT_LANE_COUNT,
  maxMessages = DEFAULT_MAX_MESSAGES,
}: DanmuOverlayProps) {
  const chat = useStore(store, (s) => s.chat);
  const segment = useStore(store, (s) => s.segment);

  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });
  const [activeDanmu, setActiveDanmu] = useState<
    Map<string, ActiveDanmuMessage>
  >(new Map());
  const processedRef = useRef(new Set<string>());
  const mountTimeRef = useRef(Date.now());
  const lastChatLenRef = useRef(0);

  const { assignLane, updateDanmuWidth, releaseLane, cleanup } = useDanmuLanes(
    laneCount,
    containerSize.width,
  );

  // Periodic cleanup of expired lane tracking
  useEffect(() => {
    const interval = setInterval(cleanup, 1000);
    return () => clearInterval(interval);
  }, [cleanup]);

  const handleMessageComplete = useCallback(
    (messageId: string) => {
      releaseLane(messageId);
      setActiveDanmu((prev) => {
        const next = new Map(prev);
        next.delete(messageId);
        return next;
      });
    },
    [releaseLane],
  );

  const handleWidthMeasured = useCallback(
    (messageId: string, width: number) => {
      updateDanmuWidth(messageId, width);
    },
    [updateDanmuWidth],
  );

  // Process new chat messages into danmu
  useEffect(() => {
    if (!enabled || containerSize.width === 0) return;

    const newCount = chat.length - lastChatLenRef.current;
    if (newCount <= 0) return;
    lastChatLenRef.current = chat.length;

    const newMessages = chat.slice(-newCount).filter((msg) => {
      if (processedRef.current.has(msg.uri)) return false;
      if (msg.author.did === "did:sys:system") return false;
      if (msg.deleted) return false;
      const msgTime = new Date(msg.record.createdAt).getTime();
      if (msgTime < mountTimeRef.current) return false;
      return true;
    });

    if (newMessages.length === 0) return;

    const messagesToAdd: ActiveDanmuMessage[] = [];

    for (const msg of newMessages.slice(0, maxMessages)) {
      if (processedRef.current.has(msg.uri)) continue;
      processedRef.current.add(msg.uri);

      if (processedRef.current.size > MAX_PROCESSED_MESSAGES) {
        const toRemove = Array.from(processedRef.current).slice(
          0,
          processedRef.current.size - MAX_PROCESSED_MESSAGES,
        );
        toRemove.forEach((uri) => processedRef.current.delete(uri));
      }

      const duration = baseDuration(msg) / speed;
      const lane = assignLane(msg.uri, duration);

      if (lane !== null) {
        messagesToAdd.push({ message: msg, lane, duration });
      }
    }

    if (messagesToAdd.length > 0) {
      setActiveDanmu((prev) => {
        const next = new Map(prev);
        for (const danmu of messagesToAdd) {
          next.set(danmu.message.uri, danmu);
        }
        return next;
      });
    }
  }, [
    chat.length,
    enabled,
    speed,
    maxMessages,
    containerSize.width,
    assignLane,
  ]);

  // Calculate video area for positioning (matching mobile layout)
  const segVideo = segment?.video?.[0];
  const videoAR = segVideo ? segVideo.width / segVideo.height : 16 / 9;
  const containerAR = containerSize.width / containerSize.height;

  let videoHeight: number;
  let videoTop: number;

  if (containerAR > videoAR) {
    // Container is wider than video - letterbox on sides
    videoHeight = containerSize.height;
    videoTop = 0;
    // Adjust for top/bottom gaps iff we don't have top letterboxing
    videoTop += TOP_GAP;
    videoHeight -= TOP_GAP + BOTTOM_GAP;
  } else {
    // Container is taller than video - letterbox on top/bottom
    videoHeight = containerSize.width / videoAR;
    videoTop = (containerSize.height - videoHeight) / 2;
  }

  const laneHeight = videoHeight / laneCount;
  const fontSize = laneHeight * FONT_SIZE_PERCENTAGE;

  if (!enabled) return null;

  return (
    <div
      className="absolute inset-0 overflow-hidden pointer-events-none"
      style={{ opacity: opacity / 100 }}
      ref={(el) => {
        if (el) {
          const rect = el.getBoundingClientRect();
          if (
            rect.width !== containerSize.width ||
            rect.height !== containerSize.height
          ) {
            setContainerSize({ width: rect.width, height: rect.height });
          }
        }
      }}
    >
      {containerSize.width > 0 &&
        Array.from(activeDanmu.entries()).map(([id, d]) => (
          <DanmuMessageElement
            key={id}
            message={d.message}
            lane={d.lane}
            laneHeight={laneHeight}
            videoTop={videoTop}
            fontSize={fontSize}
            duration={d.duration}
            containerWidth={containerSize.width}
            onComplete={handleMessageComplete}
            onWidthMeasured={handleWidthMeasured}
          />
        ))}
    </div>
  );
}

const DanmuMessageElement = memo(
  ({
    message,
    lane,
    laneHeight,
    videoTop,
    fontSize,
    duration,
    containerWidth,
    onComplete,
    onWidthMeasured,
  }: {
    message: ChatMessageViewHydrated;
    lane: number;
    laneHeight: number;
    videoTop: number;
    fontSize: number;
    duration: number;
    containerWidth: number;
    onComplete: (messageId: string) => void;
    onWidthMeasured: (messageId: string, width: number) => void;
  }) => {
    const ref = useRef<HTMLDivElement>(null);
    const measuredRef = useRef(false);

    useEffect(() => {
      if (ref.current && !measuredRef.current) {
        measuredRef.current = true;
        onWidthMeasured(message.uri, ref.current.offsetWidth);
      }
    }, [message.uri, onWidthMeasured]);

    useEffect(() => {
      const timer = setTimeout(() => {
        onComplete(message.uri);
      }, duration);
      return () => clearTimeout(timer);
    }, [message.uri, duration, onComplete]);

    const color = brightenColor(message.chatProfile?.color);

    return (
      <div
        ref={ref}
        className="absolute left-0 whitespace-nowrap font-semibold"
        style={
          {
            top: videoTop + lane * laneHeight,
            color,
            fontSize,
            textShadow: "1px 1px 2px rgba(0,0,0,0.8), 0 0 64px rgba(0,0,0,0.5)",
            animation: `danmu-scroll ${duration}ms linear forwards`,
            "--danmu-start": `${containerWidth}px`,
            willChange: "transform",
          } as React.CSSProperties
        }
      >
        {message.record.text}
      </div>
    );
  },
  (prev, next) =>
    prev.message.uri === next.message.uri &&
    prev.lane === next.lane &&
    prev.laneHeight === next.laneHeight &&
    prev.videoTop === next.videoTop &&
    prev.fontSize === next.fontSize &&
    prev.duration === next.duration &&
    prev.containerWidth === next.containerWidth,
);

DanmuMessageElement.displayName = "DanmuMessageElement";
