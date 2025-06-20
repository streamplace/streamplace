import { usePlayerStore } from "../player-store";

export function useCameraToggle() {
  const ingestCamera = usePlayerStore((x) => x.ingestCamera);
  const setIngestCamera = usePlayerStore((x) => x.setIngestCamera);

  const doSetIngestCamera = () => {
    setIngestCamera(ingestCamera === "user" ? "environment" : "user");
  };

  return { ingestCamera, doSetIngestCamera };
}
