import { useCallback, useRef } from "react";

export interface ActiveDanmu {
  id: string;
  lane: number;
  endTime: number;
  startTime: number;
  duration: number;
  width?: number;
}

const LANE_GAP = 4;

export function useDanmuLanes(laneCount: number, containerWidth: number) {
  const activeDanmu = useRef<Map<string, ActiveDanmu>>(new Map());
  const lanes = useRef(
    Array.from({ length: laneCount }, (_, i) => ({
      index: i,
      occupiedUntil: 0,
    })),
  );

  const canFitInLane = useCallback(
    (laneIndex: number): boolean => {
      const now = Date.now();

      const danmuInLane = Array.from(activeDanmu.current.values()).filter(
        (d) => d.lane === laneIndex && d.endTime > now,
      );

      if (danmuInLane.length === 0) return true;

      const mostRecent = danmuInLane.reduce((latest, current) =>
        current.startTime > latest.startTime ? current : latest,
      );

      const elapsed = now - mostRecent.startTime;
      const progress = elapsed / mostRecent.duration;
      const traveled = containerWidth * progress;

      const estimatedWidth = mostRecent.width || 200;
      const spaceNeeded = estimatedWidth + LANE_GAP;

      return traveled >= spaceNeeded;
    },
    [containerWidth],
  );

  const assignLane = useCallback(
    (messageId: string, duration: number, width?: number): number | null => {
      const now = Date.now();

      for (const lane of lanes.current) {
        if (canFitInLane(lane.index)) {
          const endTime = now + duration;
          activeDanmu.current.set(messageId, {
            id: messageId,
            lane: lane.index,
            endTime,
            startTime: now,
            duration,
            width,
          });
          return lane.index;
        }
      }

      return null;
    },
    [lanes, canFitInLane],
  );

  const updateDanmuWidth = useCallback((messageId: string, width: number) => {
    const danmu = activeDanmu.current.get(messageId);
    if (danmu) {
      danmu.width = width;
    }
  }, []);

  const releaseLane = useCallback((messageId: string) => {
    activeDanmu.current.delete(messageId);
  }, []);

  const cleanup = useCallback(() => {
    const now = Date.now();
    for (const [id, danmu] of activeDanmu.current.entries()) {
      if (danmu.endTime <= now) {
        activeDanmu.current.delete(id);
      }
    }
  }, []);

  return {
    assignLane,
    updateDanmuWidth,
    releaseLane,
    cleanup,
  };
}
