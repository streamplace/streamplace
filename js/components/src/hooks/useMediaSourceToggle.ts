import { usePlayerStore } from "../player-store";
import { IngestMediaSource } from "../player-store/player-state";

export function useMediaSourceToggle() {
  const ingestMediaSource = usePlayerStore((x) => x.ingestMediaSource);
  const setIngestMediaSource = usePlayerStore((x) => x.setIngestMediaSource);

  const toggleMediaSource = () => {
    if (ingestMediaSource === IngestMediaSource.DISPLAY) {
      setIngestMediaSource(undefined); // back to camera
    } else {
      setIngestMediaSource(IngestMediaSource.DISPLAY); // screen share
    }
  };

  return { ingestMediaSource, toggleMediaSource };
}
