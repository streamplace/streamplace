import { place } from "streamplace";
import {
  ContentMetadataResult,
  useDID,
  useSetContentMetadata,
  useStreamplaceStore,
} from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

export const useGetBroadcasterDID = () => {
  const pdsAgent = usePDSAgent();
  const did = useDID();
  const setBroadcasterDID = useStreamplaceStore(
    (state) => state.setBroadcasterDID,
  );
  const setServerDID = useStreamplaceStore((state) => state.setServerDID);
  return async () => {
    if (!pdsAgent || !did) {
      throw new Error("No PDS agent or DID available");
    }

    const result = await pdsAgent.client.call(
      place.stream.broadcast.getBroadcaster,
    );
    setBroadcasterDID(result.broadcaster);
    if (result.server) {
      setServerDID(result.server);
    } else {
      setServerDID(null);
    }
  };
};

export const useSaveContentMetadata = () => {
  const pdsAgent = usePDSAgent();
  const did = useDID();
  const setContentMetadata = useSetContentMetadata();

  return async (
    metadataRecord: place.stream.metadata.configuration.Main,
    previous: ContentMetadataResult | null,
  ) => {
    if (!pdsAgent || !did) {
      throw new Error("No PDS agent or DID available");
    }

    const result = previous
      ? await pdsAgent.client.put(
          place.stream.metadata.configuration,
          metadataRecord,
          { repo: did as any, rkey: "self", swapRecord: previous.cid },
        )
      : await pdsAgent.client.create(
          place.stream.metadata.configuration,
          metadataRecord,
          { repo: did as any, rkey: "self" },
        );

    const contentMetadata: ContentMetadataResult = {
      record: metadataRecord as any,
      uri: result.uri,
      cid: result.cid || "",
      rkey: "self",
    };
    setContentMetadata(contentMetadata);
    return contentMetadata;
  };
};

// Simple get function
export const useGetContentMetadata = () => {
  const pdsAgent = usePDSAgent();
  const did = useDID();
  const setContentMetadata = useSetContentMetadata();

  return async (params?: { userDid?: string; rkey?: string }) => {
    if (!pdsAgent) {
      throw new Error("No PDS agent available");
    }

    const targetDid = params?.userDid || did;
    if (!targetDid) {
      throw new Error("No DID provided or user not authenticated");
    }

    try {
      const result = await pdsAgent.client.get(
        place.stream.metadata.configuration,
        {
          repo: targetDid as any,
          rkey: (params?.rkey || "self") as "self",
        },
      );

      const contentMetadata: ContentMetadataResult = {
        record: result.value,
        uri: result.uri,
        cid: result.cid || "",
      };

      setContentMetadata(contentMetadata);
      return contentMetadata;
    } catch (error) {
      // Handle record not found - this is normal for new users
      if (
        error instanceof Error &&
        (("error" in error && error.error === "RecordNotFound") ||
          error.message?.includes("not found") ||
          error.message?.includes("RecordNotFound") ||
          error.message?.includes("mst: not found") ||
          (error as any)?.status === 404)
      ) {
        return null;
      }
      throw error;
    }
  };
};
