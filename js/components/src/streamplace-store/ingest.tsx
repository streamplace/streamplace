import { place } from "streamplace";
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

    const result = await pdsAgent.client.call(
      place.stream.ingest.getIngestUrls,
    );

    const ingests = result.ingests
      .map((ingest) => {
        if (place.stream.ingest.defs.ingest.isTypeOf(ingest)) {
          return ingest;
        }
        console.error("Invalid ingest", ingest);
        return null;
      })
      .filter((ingest) => ingest !== null);

    setIngests(ingests);
  };
}
