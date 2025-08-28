import {
  ContentMetadataResult,
  useDID,
  useSetContentMetadata,
} from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

// Consolidated function for both create and update operations (uses putRecord for both)
export const useSaveContentMetadata = () => {
  const pdsAgent = usePDSAgent();
  const did = useDID();
  const setContentMetadata = useSetContentMetadata();

  return async (params: {
    rkey?: string;
    livestreamRef?: { uri: string; cid: string };
    contentWarnings?: string[];
    distributionPolicy?: { deleteAfter?: string };
    contentRights?: Record<string, any>;
    // Internal flag to track if this is an update operation for loading states
    _isUpdate?: boolean;
  }) => {
    if (!pdsAgent || !did) {
      throw new Error("No PDS agent or DID available");
    }

    const isUpdate = params._isUpdate || false;
    setContentMetadata({
      [isUpdate ? "updating" : "creating"]: true,
      error: null,
    });

    try {
      const metadataRecord = {
        $type: "place.stream.metadata.configuration",
        ...(params.livestreamRef && { livestreamRef: params.livestreamRef }),
        createdAt: new Date().toISOString(),
        ...(params.contentWarnings?.length && {
          contentWarnings: { warnings: params.contentWarnings },
        }),
        ...(params.distributionPolicy?.deleteAfter && {
          distributionPolicy: params.distributionPolicy,
        }),
        ...(params.contentRights &&
          Object.keys(params.contentRights).length > 0 && {
            contentRights: params.contentRights,
          }),
      };

      const result = await (pdsAgent as any).com.atproto.repo.putRecord({
        repo: did,
        collection: "place.stream.metadata.configuration",
        rkey: params.rkey || "self",
        record: metadataRecord,
      });

      const contentMetadata: ContentMetadataResult = {
        record: metadataRecord as any,
        uri: result.data.uri,
        cid: result.data.cid || "",
        rkey: params.rkey || "self",
      };

      setContentMetadata({
        contentMetadata,
        [isUpdate ? "updating" : "creating"]: false,
        error: null,
      });

      return contentMetadata;
    } catch (error) {
      const errorMessage =
        error instanceof Error
          ? error.message
          : "Failed to save content metadata";
      setContentMetadata({
        error: errorMessage,
        [isUpdate ? "updating" : "creating"]: false,
      });
      throw error;
    }
  };
};

// Legacy aliases for backward compatibility
export const useCreateContentMetadata = () => {
  const saveContentMetadata = useSaveContentMetadata();
  return (params: {
    contentWarnings?: string[];
    distributionPolicy?: { deleteAfter?: string };
    contentRights?: Record<string, any>;
  }) => saveContentMetadata({ ...params, _isUpdate: false });
};

export const useUpdateContentMetadata = () => {
  const saveContentMetadata = useSaveContentMetadata();
  return (params: {
    rkey?: string;
    livestreamRef?: { uri: string; cid: string };
    contentWarnings?: string[];
    distributionPolicy?: { deleteAfter?: string };
    contentRights?: Record<string, any>;
  }) => saveContentMetadata({ ...params, _isUpdate: true });
};

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

    setContentMetadata({ error: null });

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

      setContentMetadata({ contentMetadata });
      return contentMetadata;
    } catch (error) {
      // Handle record not found cases (this is expected for new users)
      // This includes both proper 404 responses and backend 500 errors for missing records
      if (
        error instanceof Error &&
        (error.message?.includes("not found") ||
          error.message?.includes("RecordNotFound") ||
          error.message?.includes("mst: not found") ||
          // Handle HTTP error responses that indicate "not found"
          (error as any)?.status === 404 ||
          (error as any)?.response?.status === 404 ||
          // Handle backend 500 errors that are actually "not found" scenarios
          ((error as any)?.status === 500 && error.message?.includes("mst")))
      ) {
        // Record doesn't exist yet, this is normal for new users
        console.debug(
          "Content metadata record not found (expected for new users)",
        );
        // Don't clear existing contentMetadata state - user might have just created a record
        // Only clear error state
        setContentMetadata({ error: null });
        return null;
      }

      // Handle other errors
      const errorMessage =
        error instanceof Error
          ? error.message
          : "Failed to get content metadata";
      console.warn("Content metadata fetch error:", errorMessage);
      setContentMetadata({ error: errorMessage });
      throw error;
    }
  };
};

export const useClearContentMetadataError = () => {
  const setContentMetadata = useSetContentMetadata();
  return () => setContentMetadata({ error: null });
};
