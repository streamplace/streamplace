import { place, VideoViewHydrated } from "streamplace";
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

function getVideoMetadata(view: VideoViewHydrated) {
  if (!view.tracks) {
    return null;
  }
  for (const track of view.tracks) {
    const meta = track.record.metadata;
    if (!meta || !place.stream.media.track.commonMetadata.isTypeOf(meta)) {
      continue;
    }
    const common = meta as place.stream.media.track.CommonMetadata;
    if (!common.video) {
      continue;
    }
    return common.video;
  }
  return null;
}
