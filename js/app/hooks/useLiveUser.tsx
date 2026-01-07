import { useStore } from "store";
import { PlaceStreamSegment } from "streamplace";

// composite selector that tells us when the current user is live
export const useLiveUser = (): boolean => {
  const mySegments = useStore((state) => state.mySegments);
  if (mySegments.length === 0) {
    return false;
  }
  if (!PlaceStreamSegment.isRecord(mySegments[0].record)) {
    return false;
  }
  const record = mySegments[0].record as PlaceStreamSegment.Record;
  if (Date.now() - new Date(record.startTime).getTime() < 1000 * 10) {
    return true;
  }
  return false;
};
