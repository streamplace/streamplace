import { selectMySegments } from "features/streamplace/streamplaceSlice";
import { isRecord } from "lexicons/types/place/stream/segment";
import { useAppSelector } from "store/hooks";
import { PlaceStreamSegment } from "lexicons";

// composite selector that tells us when the current user is live
export const useLiveUser = (): boolean => {
  const mySegments = useAppSelector(selectMySegments);
  if (mySegments.length === 0) {
    return false;
  }
  if (!isRecord(mySegments[0].record)) {
    return false;
  }
  const record = mySegments[0].record as PlaceStreamSegment.Record;
  console.log(new Date(record.startTime).getTime() - Date.now());
  if (Date.now() - new Date(record.startTime).getTime() < 1000 * 10) {
    console.log("user is live", record.startTime);
    return true;
  }
  console.log("user is not live", record.startTime);
  return false;
};
