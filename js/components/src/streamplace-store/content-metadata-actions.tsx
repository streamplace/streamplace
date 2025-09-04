import {
  ContentMetadataResult,
  useDID,
  useSetContentMetadata,
} from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

export const useSaveContentMetadata = () => {
  const pdsAgent = usePDSAgent();
  const did = useDID();
  const setContentMetadata = useSetContentMetadata();

  return async (params: {
    contentWarnings?: string[];
    distributionPolicy?: { deleteAfter?: string };
    contentRights?: Record<string, any>;
    rkey?: string;
  }) => {
    if (!pdsAgent || !did) {
      throw new Error("No PDS agent or DID available");
    }

    const metadataRecord = {
      $type: "place.stream.metadata.configuration",
      createdAt: new Date().toISOString(),
      ...(params.contentWarnings &&
        params.contentWarnings.length > 0 && {
          contentWarnings: { warnings: params.contentWarnings },
        }),
      ...(params.distributionPolicy &&
        params.distributionPolicy.deleteAfter && {
          distributionPolicy: params.distributionPolicy,
        }),
      ...(params.contentRights &&
        Object.keys(params.contentRights).length > 0 && {
          contentRights: params.contentRights,
        }),
    };

    const rkey = params.rkey || "self";

    try {
      // Try to update existing record first
      const result = await (pdsAgent as any).com.atproto.repo.putRecord({
        repo: did,
        collection: "place.stream.metadata.configuration",
        rkey,
        record: metadataRecord,
      });

      const contentMetadata: ContentMetadataResult = {
        record: metadataRecord as any,
        uri: result.data.uri,
        cid: result.data.cid || "",
        rkey,
      };

      setContentMetadata(contentMetadata);
      return contentMetadata;
    } catch (error) {
      // If record doesn't exist, create it
      if (
        error instanceof Error &&
        (error.message?.includes("not found") ||
          error.message?.includes("RecordNotFound") ||
          error.message?.includes("mst: not found") ||
          (error as any)?.status === 404)
      ) {
        const createResult = await (
          pdsAgent as any
        ).com.atproto.repo.createRecord({
          repo: did,
          collection: "place.stream.metadata.configuration",
          rkey,
          record: metadataRecord,
        });

        const contentMetadata: ContentMetadataResult = {
          record: metadataRecord as any,
          uri: createResult.data.uri,
          cid: createResult.data.cid || "",
          rkey,
        };

        setContentMetadata(contentMetadata);
        return contentMetadata;
      }
      throw error;
    }
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
      const result = await (pdsAgent as any).com.atproto.repo.getRecord({
        repo: targetDid,
        collection: "place.stream.metadata.configuration",
        rkey: params?.rkey || "self",
      });

      if (!result.success) {
        throw new Error("Failed to get content metadata record");
      }

      const contentMetadata: ContentMetadataResult = {
        record: result.data.value,
        uri: result.data.uri,
        cid: result.data.cid || "",
      };

      setContentMetadata(contentMetadata);
      return contentMetadata;
    } catch (error) {
      // Handle record not found - this is normal for new users
      if (
        error instanceof Error &&
        (error.message?.includes("not found") ||
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
