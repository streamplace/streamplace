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

export function vodFrameRateForQuality(
  levels: VodLevel[],
  quality: string,
  playingRendition: string | null,
): number {
  const rendition = quality === "source" ? playingRendition : quality;
  const frameRate = levels.find((level) => level.name === rendition)?.frameRate;
  return frameRate && Number.isFinite(frameRate) && frameRate > 0
    ? frameRate
    : 30;
}
