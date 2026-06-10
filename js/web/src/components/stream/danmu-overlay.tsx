import { getRgbColor } from "@/lib/color";
import type { LivestreamStore } from "@streamplace/core";
import { memo, useCallback, useEffect, useRef, useState } from "react";
import type { ChatMessageViewHydrated } from "streamplace";
import { useStore } from "zustand";

// Duration constants (ms). Text length maps to a scroll duration
// between MIN and MAX so short messages fly by faster.
const MIN_DURATION = 6000;
const MAX_DURATION = 12000;
const DEFAULT_LANE_COUNT = 12;
const DEFAULT_OPACITY = 80;
const DEFAULT_SPEED = 1;
const DEFAULT_MAX_MESSAGES = 50;
const FONT_SIZE_PERCENTAGE = 0.7;
const TOP_GAP = 20;
const BOTTOM_GAP = 60;

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

interface DanmuOverlayProps {
  store: LivestreamStore;
  enabled?: boolean;
  opacity?: number;
  speed?: number;
  laneCount?: number;
  maxMessages?: number;
}

interface ActiveDanmu {
  id: string;
  message: ChatMessageViewHydrated;
  lane: number;
  duration: number;
  width: number;
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
  const [activeDanmu, setActiveDanmu] = useState<ActiveDanmu[]>([]);
  const processedRef = useRef(new Set<string>());
  const mountTimeRef = useRef(Date.now());
  const lastChatLenRef = useRef(0);
  const idCounterRef = useRef(0);
  const laneOccupancyRef = useRef<number[]>(new Array(laneCount).fill(0));

  const assignLane = useCallback((): number => {
    const occ = laneOccupancyRef.current;
    let minLane = 0;
    let minVal = occ[0];
    for (let i = 1; i < occ.length; i++) {
      if (occ[i] < minVal) {
        minVal = occ[i];
        minLane = i;
      }
    }
    occ[minLane]++;
    return minLane;
  }, []);

  const releaseLane = useCallback((lane: number) => {
    const occ = laneOccupancyRef.current;
    if (occ[lane] > 0) occ[lane]--;
  }, []);

  // Process new chat messages into danmu
  useEffect(() => {
    if (!enabled || containerSize.width === 0) return;

    const newCount = chat.length - lastChatLenRef.current;
    if (newCount <= 0) return;
    lastChatLenRef.current = chat.length;

    const newMessages = chat.slice(0, newCount).filter((msg) => {
      if (processedRef.current.has(msg.uri)) return false;
      if (msg.author.did === "did:sys:system") return false;
      if (msg.deleted) return false;
      const msgTime = new Date(msg.record.createdAt).getTime();
      if (msgTime < mountTimeRef.current) return false;
      return true;
    });

    if (newMessages.length === 0) return;

    const toAdd: ActiveDanmu[] = [];
    for (const msg of newMessages.slice(0, 10)) {
      if (processedRef.current.has(msg.uri)) continue;
      processedRef.current.add(msg.uri);

      // Prune old processed entries
      if (processedRef.current.size > 200) {
        const iter = processedRef.current.values();
        for (let i = 0; i < 100; i++) {
          const val = iter.next().value;
          if (val) processedRef.current.delete(val);
        }
      }

      const dur = baseDuration(msg) / speed;
      const lane = assignLane();
      toAdd.push({
        id: `danmu-${++idCounterRef.current}`,
        message: msg,
        lane,
        duration: dur,
        width: 0,
      });
    }

    if (toAdd.length > 0) {
      setActiveDanmu((prev) => [...prev, ...toAdd].slice(-maxMessages));

      // Schedule cleanup
      for (const item of toAdd) {
        setTimeout(() => {
          releaseLane(item.lane);
          setActiveDanmu((prev) => prev.filter((d) => d.id !== item.id));
        }, item.duration);
      }
    }
  }, [
    chat.length,
    enabled,
    speed,
    maxMessages,
    containerSize.width,
    assignLane,
    releaseLane,
  ]);

  // Calculate video area for positioning
  const segVideo = segment?.video?.[0];
  const videoAR = segVideo ? segVideo.width / segVideo.height : 16 / 9;
  const containerAR = containerSize.width / containerSize.height;

  let videoWidth: number;
  let videoHeight: number;
  let videoTop: number;

  if (containerAR > videoAR) {
    videoHeight = containerSize.height;
    videoWidth = videoHeight * videoAR;
    videoTop = 0;
  } else {
    videoWidth = containerSize.width;
    videoHeight = videoWidth / videoAR;
    videoTop = (containerSize.height - videoHeight) / 2;
  }

  videoTop += TOP_GAP;
  videoHeight -= TOP_GAP + BOTTOM_GAP;

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
        activeDanmu.map((d) => (
          <DanmuMessageElement
            key={d.id}
            message={d.message}
            lane={d.lane}
            laneHeight={laneHeight}
            videoTop={videoTop}
            fontSize={fontSize}
            duration={d.duration}
            containerWidth={containerSize.width}
            onWidthMeasured={(w) => {
              setActiveDanmu((prev) =>
                prev.map((item) =>
                  item.id === d.id ? { ...item, width: w } : item,
                ),
              );
            }}
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
    onWidthMeasured,
  }: {
    message: ChatMessageViewHydrated;
    lane: number;
    laneHeight: number;
    videoTop: number;
    fontSize: number;
    duration: number;
    containerWidth: number;
    onWidthMeasured: (width: number) => void;
  }) => {
    const ref = useRef<HTMLDivElement>(null);
    const measuredRef = useRef(false);

    useEffect(() => {
      if (ref.current && !measuredRef.current) {
        measuredRef.current = true;
        onWidthMeasured(ref.current.offsetWidth);
      }
    }, [onWidthMeasured]);

    const color = getRgbColor(message.chatProfile?.color);
    const totalDistance = containerWidth + (ref.current?.offsetWidth ?? 200);

    return (
      <div
        ref={ref}
        className="absolute left-0 whitespace-nowrap font-semibold"
        style={
          {
            top: videoTop + lane * laneHeight,
            color,
            fontSize,
            textShadow: "1px 1px 2px rgba(0,0,0,0.8), 0 0 8px rgba(0,0,0,0.5)",
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
