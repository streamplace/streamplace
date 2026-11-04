import { describe, expect, it } from "vitest";
import {
  vodLevelsFromHls,
  vodQualityChoices,
} from "../src/components/mobile-player/vod-levels";

describe("VOD rendition metadata", () => {
  it("keeps single-rendition frame-rate metadata without offering redundant quality choices", () => {
    const hlsLevels = [{ height: 1080, bitrate: 4_000_000, frameRate: 60 }];
    const levels = vodLevelsFromHls("vod", hlsLevels);

    expect(levels).toEqual([{ name: "1080p", frameRate: 60 }]);
    expect(vodQualityChoices(levels)).toEqual([]);
    expect(vodLevelsFromHls("live", hlsLevels)).toEqual([]);
  });

  it("offers quality choices when the manifest has multiple renditions", () => {
    const levels = vodLevelsFromHls("vod", [
      { height: 1080, bitrate: 4_000_000, frameRate: 60 },
      { height: 0, bitrate: 600_000, frameRate: 30 },
    ]);

    expect(vodQualityChoices(levels)).toEqual([
      { name: "1080p", frameRate: 60 },
      { name: "600k", frameRate: 30 },
    ]);
  });
});
