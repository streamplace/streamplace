import { PlaceStreamIngestDefs } from "streamplace";
import { useDID, useStreamplaceStore } from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

export default function useGetIngests() {
  const pdsAgent = usePDSAgent();
  const did = useDID();
  const setIngests = useStreamplaceStore((state) => state.setIngests);

  return async () => {
    if (!pdsAgent || !did) {
      throw new Error("No PDS agent or DID available");
    }

    const result = await pdsAgent.place.stream.ingest.getIngestUrls();
    if (!result.success) {
      throw new Error("Failed to get ingests");
    }

    const ingests = result.data.ingests
      .map((ingest) => {
        if (PlaceStreamIngestDefs.isIngest(ingest)) {
          return ingest;
        }
        console.error("Invalid ingest", ingest);
        return null;
      })
      .filter((ingest) => ingest !== null);

    setIngests(ingests);
  };
}
