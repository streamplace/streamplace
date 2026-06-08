// Composite selector that tells the caller when the authenticated
// user is currently broadcasting. Mirrors js/app/hooks/useLiveUser.
import { PlaceStreamSegment } from "streamplace";
import { useMySegments } from "../lib/store/hooks";

export function useLiveUser(): boolean {
  const mySegments = useMySegments();
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
}
