import type { PlayerMode, VodLevel } from "../../player-store/player-state";

type HlsLevelDetails = {
  height: number;
  bitrate: number;
  frameRate?: number;
};

export function vodLevelsFromHls(
  mode: PlayerMode,
  levels: HlsLevelDetails[],
): VodLevel[] {
  if (mode !== "vod") return [];
  return levels.map((level) => ({
    name:
      level.height > 0
        ? `${level.height}p`
        : `${Math.round(level.bitrate / 1000)}k`,
    frameRate: level.frameRate,
  }));
}

export function vodQualityChoices(levels: VodLevel[]): VodLevel[] {
  return levels.length > 1 ? levels : [];
}
