import { useCallback, useEffect, useRef, useState } from "react";
import { LayoutChangeEvent, StyleSheet } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { useChat, useLivestreamStore } from "../../livestream-store";
import { View } from "../ui";
import { DanmuMessage } from "./danmu-message";
import { baseDuration, MAX_DURATION, MIN_DURATION } from "./math";
import { useDanmuLanes } from "./use-danmu-lanes";

interface DanmuOverlayProps {
  enabled?: boolean;
  opacity?: number;
  speed?: number;
  laneCount?: number;
  maxMessages?: number;
}

interface ActiveDanmuMessage {
  message: ChatMessageViewHydrated;
  lane: number;
}

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

export function DanmuOverlay({
  enabled = true,
  opacity = DEFAULT_OPACITY,
  speed = DEFAULT_SPEED,
  laneCount = DEFAULT_LANE_COUNT,
  maxMessages = DEFAULT_MAX_MESSAGES,
}: DanmuOverlayProps) {
  const chat = useChat();
  const segment = useLivestreamStore((x) => x.segment);

  const [containerWidth, setContainerWidth] = useState(0);
  const [containerHeight, setContainerHeight] = useState(0);
  const [activeDanmu, setActiveDanmu] = useState<
    Map<string, ActiveDanmuMessage>
  >(new Map());
  const processedMessages = useRef(new Set<string>());
  const mountTime = useRef(Date.now());
  const lastChatLength = useRef(0);
  const { assignLane, updateDanmuWidth, releaseLane, cleanup } = useDanmuLanes(
    laneCount,
    containerWidth,
  );

  const handleLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setContainerWidth(width);
    setContainerHeight(height);
  }, []);

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

  useEffect(() => {
    if (!enabled || !chat || containerWidth === 0) return;

    // only check new messages since last render (chat is sorted newest first)
    const newMessageCount = chat.length - lastChatLength.current;
    if (newMessageCount <= 0) return;

    const messagesToCheck = chat.slice(0, newMessageCount);
    lastChatLength.current = chat.length;

    const newMessages = messagesToCheck.filter((msg) => {
      const hasProcessed = processedMessages.current.has(msg.uri);
      const isSystem = msg.author.did === "did:sys:system";
      const msgTime = new Date(msg.record.createdAt).getTime();
      const isAfterMount = msgTime >= mountTime.current;

      return !hasProcessed && !isSystem && isAfterMount;
    });

    if (newMessages.length === 0) return;

    const messagesToAdd: ActiveDanmuMessage[] = [];

    for (const message of newMessages.slice(0, maxMessages)) {
      // mark as processed FIRST to prevent duplicate processing if effect runs twice
      if (processedMessages.current.has(message.uri)) {
        continue;
      }
      processedMessages.current.add(message.uri);

      if (processedMessages.current.size > MAX_PROCESSED_MESSAGES) {
        const toRemove = Array.from(processedMessages.current).slice(
          0,
          processedMessages.current.size - MAX_PROCESSED_MESSAGES,
        );
        toRemove.forEach((uri) => processedMessages.current.delete(uri));
      }

      const duration = baseDuration(message, MAX_DURATION, MIN_DURATION);
      if (__DEV__)
        console.log("[danmu] message", message.record.text, {
          duration,
          speed,
        });
      const lane = assignLane(message.uri, duration);

      if (lane !== null) {
        messagesToAdd.push({ message, lane });
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
  }, [chat, enabled, speed, maxMessages]);

  useEffect(() => {
    const interval = setInterval(() => {
      cleanup();
    }, 1000);

    return () => clearInterval(interval);
  }, [cleanup]);

  if (!enabled || containerWidth === 0 || containerHeight === 0) {
    return <View style={styles.container} onLayout={handleLayout} />;
  }

  // Calculate video bounds based on actual video dimensions from segment
  const segmentVideoWidth = segment?.video?.[0]?.width;
  const segmentVideoHeight = segment?.video?.[0]?.height;

  // Fall back to 16:9 if no segment dimensions available
  const videoAspectRatio =
    segmentVideoWidth && segmentVideoHeight
      ? segmentVideoWidth / segmentVideoHeight
      : 16 / 9;

  const containerAspectRatio = containerWidth / containerHeight;

  let videoWidth: number;
  let videoHeight: number;
  let videoTop: number;
  let videoLeft: number;

  if (containerAspectRatio > videoAspectRatio) {
    // Container is wider than video - letterbox on sides
    videoHeight = containerHeight;
    videoWidth = videoHeight * videoAspectRatio;
    videoTop = 0;
    videoLeft = (containerWidth - videoWidth) / 2;
    // Adjust for top/bottom gaps iff we don't have top letterboxing
    videoTop += TOP_GAP;
    videoHeight -= TOP_GAP + BOTTOM_GAP;
  } else {
    // Container is taller than video - letterbox on top/bottom
    videoWidth = containerWidth;
    videoHeight = videoWidth / videoAspectRatio;
    videoTop = (containerHeight - videoHeight) / 2;
    videoLeft = 0;
  }

  const laneHeight = videoHeight / laneCount;
  const fontSize = laneHeight * FONT_SIZE_PERCENTAGE;

  return (
    <View style={styles.container} onLayout={handleLayout} pointerEvents="none">
      {Array.from(activeDanmu.values()).map(({ message, lane }) => (
        <DanmuMessage
          key={message.uri}
          message={message}
          lane={lane}
          laneHeight={laneHeight}
          videoTop={videoTop}
          opacity={opacity}
          fontSize={fontSize}
          speed={speed}
          containerWidth={containerWidth}
          containerHeight={containerHeight}
          onComplete={handleMessageComplete}
          onWidthMeasured={handleWidthMeasured}
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    overflow: "hidden",
  },
});
