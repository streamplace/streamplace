import { isSegmentFresh } from "@streamplace/core";
import { useStore } from "store";
import { place } from "streamplace";

// composite selector that tells us when the current user is live
export const useLiveUser = (): boolean => {
  const mySegments = useStore((state) => state.mySegments);
  if (mySegments.length === 0) {
    return false;
  }
  if (!place.stream.segment.$isTypeOf(mySegments[0].record)) {
    return false;
  }
  const record = mySegments[0].record as place.stream.segment.Main;
  return isSegmentFresh(record.startTime, Date.now());
};
