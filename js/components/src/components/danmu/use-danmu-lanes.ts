import { useCallback, useRef } from "react";

export interface DanmuLane {
  index: number;
  occupiedUntil: number;
}

export interface ActiveDanmu {
  id: string;
  lane: number;
  endTime: number;
  startTime: number;
  duration: number;
  width?: number;
}

const LANE_GAP = 4;

export const useDanmuLanes = (laneCount: number, containerWidth: number) => {
  const activeDanmu = useRef<Map<string, ActiveDanmu>>(new Map());
  const lanes = useRef<DanmuLane[]>(
    Array.from({ length: laneCount }, (_, i) => ({
      index: i,
      occupiedUntil: 0,
    })),
  );

  const canFitInLane = useCallback(
    (laneIndex: number): boolean => {
      const now = Date.now();

      // find all active danmu in this lane
      const danmuInLane = Array.from(activeDanmu.current.values()).filter(
        (d) => d.lane === laneIndex && d.endTime > now,
      );

      if (danmuInLane.length === 0) return true;

      // check the most recent danmu in this lane
      const mostRecent = danmuInLane.reduce((latest, current) =>
        current.startTime > latest.startTime ? current : latest,
      );

      // calculate how far it has traveled
      const elapsed = now - mostRecent.startTime;
      const progress = elapsed / mostRecent.duration;
      const traveled = containerWidth * progress;

      // estimate width (assume ~8px per char, will be updated with actual width later)
      const estimatedWidth = mostRecent.width || 200;

      // check if there's enough space
      const spaceNeeded = estimatedWidth + LANE_GAP;
      const hasSpace = traveled >= spaceNeeded;

      return hasSpace;
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
    const danmu = activeDanmu.current.get(messageId);
    if (danmu) {
      activeDanmu.current.delete(messageId);
    }
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
};
