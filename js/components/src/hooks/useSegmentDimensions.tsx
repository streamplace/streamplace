import {
  PlaceStreamMediaTrack,
  PlaceStreamSegment,
  VideoViewHydrated,
} from "streamplace";
import { useLivestreamStoreOptional } from "../livestream-store";
import { useVideoStoreOptional } from "../video-store";

export function useSegmentDimensions() {
  const video = useVideoStoreOptional((x) => x.video);
  const latestSegment = useLivestreamStoreOptional((x) => x.segment);

  let height = 0;
  let width = 0;
  if (video) {
    const meta = getVideoMetadata(video);
    if (meta) {
      height = meta.height || 0;
      width = meta.width || 0;
    }
  } else {
    let seg = latestSegment?.video && latestSegment.video[0];

    height = seg?.height || 0;
    width = seg?.width || 0;
  }

  return {
    isPlayerRatioGreater: width > height,
    height: height,
    width: width,
  };
}

function getVideoMetadata(
  view: VideoViewHydrated,
): PlaceStreamSegment.Video | null {
  if (!view.tracks) {
    return null;
  }
  for (const track of view.tracks) {
    const meta = track.record.metadata;
    if (!PlaceStreamMediaTrack.isCommonMetadata(meta)) {
      continue;
    }
    if (!meta.video) {
      continue;
    }
    return meta.video;
  }
  return null;
}
