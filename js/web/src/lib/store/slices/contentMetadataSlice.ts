import type { CaptionPolicy } from "@streamplace/core";
import type { place } from "streamplace";
import { StateCreator } from "zustand";
import { getPDSServiceEndpoint, resolveDIDDocument } from "../../did";
import { AppStore } from "../index";

export interface ContentMetadataSlice {
  saving: boolean;
  error: string | null;
  lastCreatedRecord: any | null;
  // actions
  saveContentMetadata: (params: {
    captionPolicy?: CaptionPolicy;
    rkey?: string;
    livestreamRef?: { uri: string; cid: string };
    contentWarnings?: string[];
    distributionPolicy?: {
      deleteAfter?: number;
      allowGenAiTraining?: boolean;
      allowedBroadcasters?: string[];
    };
    contentRights?: {
      creator?: string;
      copyrightNotice?: string;
      copyrightYear?: number;
      license?: string;
      creditLine?: string;
    };
  }) => Promise<void>;
  getContentMetadata: (params?: {
    userDid?: string;
    rkey?: string;
  }) => Promise<void>;
  clearError: () => void;
}

function mergeMetadata(
  existing: place.stream.metadata.configuration.Main | undefined,
  params: Parameters<ContentMetadataSlice["saveContentMetadata"]>[0],
) {
  return {
    ...existing,
    $type: "place.stream.metadata.configuration",
    createdAt: new Date().toISOString(),
    ...(params.captionPolicy && { captionPolicy: params.captionPolicy }),
    ...(params.livestreamRef && { livestreamRef: params.livestreamRef }),
    ...(params.contentWarnings !== undefined && {
      contentWarnings: {
        ...existing?.contentWarnings,
        warnings: params.contentWarnings,
      },
    }),
    ...(params.distributionPolicy && {
      distributionPolicy: {
        ...existing?.distributionPolicy,
        ...params.distributionPolicy,
      },
    }),
    ...(params.contentRights && {
      contentRights: { ...existing?.contentRights, ...params.contentRights },
    }),
  };
}

export const createContentMetadataSlice: StateCreator<
  AppStore,
  [],
  [],
  ContentMetadataSlice
> = (set, get) => ({
  saving: false,
  error: null,
  lastCreatedRecord: null,

  saveContentMetadata: async ({
    rkey = "self",
    livestreamRef,
    contentWarnings,
    distributionPolicy,
    contentRights,
    captionPolicy,
  }) => {
    set({ saving: true, error: null });
    try {
      const { pdsAgent, oauthSession } = get();
      if (!pdsAgent) {
        throw new Error("No agent");
      }

      const did = oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }

      const metadataRecord = mergeMetadata(get().lastCreatedRecord?.record, {
        livestreamRef,
        contentWarnings,
        distributionPolicy,
        contentRights,
        captionPolicy,
      });

      const result = await pdsAgent.com.atproto.repo.putRecord({
        repo: did,
        collection: "place.stream.metadata.configuration",
        rkey,
        record: metadataRecord,
      });

      set({
        saving: false,
        error: null,
        lastCreatedRecord: {
          record: metadataRecord,
          uri: result.data.uri,
          rkey,
          cid: result.data.cid,
        },
      });
    } catch (error: any) {
      set({
        saving: false,
        error: error?.message ?? "Failed to save content metadata",
      });
      throw error;
    }
  },

  getContentMetadata: async ({ userDid, rkey = "self" } = {}) => {
    set({ error: null });
    try {
      const { pdsAgent, oauthSession } = get();
      if (!pdsAgent) {
        throw new Error("No agent");
      }

      const targetDid = userDid || oauthSession?.did;
      if (!targetDid) {
        throw new Error("No DID provided or user not authenticated");
      }

      try {
        let targetPDS: string | null = null;
        try {
          const didDoc = await resolveDIDDocument(targetDid);
          targetPDS = getPDSServiceEndpoint(didDoc);
        } catch {
          // best-effort: fall through to default agent
        }

        let agent = pdsAgent;
        if (targetPDS && targetPDS !== (pdsAgent as any)?.host) {
          const { StreamplaceAgent } = await import("streamplace");
          agent = new StreamplaceAgent(targetPDS) as any;
        }

        const result = await agent.com.atproto.repo.getRecord({
          repo: targetDid,
          collection: "place.stream.metadata.configuration",
          rkey,
        });

        if (!result.success) {
          throw new Error("Failed to get content metadata record");
        }

        set({
          error: null,
          lastCreatedRecord: {
            userDid: targetDid,
            record: result.data.value,
            uri: result.data.uri,
            cid: result.data.cid,
          },
        });
      } catch (error: any) {
        if (
          error.message?.includes("not found") ||
          error.message?.includes("RecordNotFound")
        ) {
          set({
            error: null,
            lastCreatedRecord: {
              userDid: targetDid,
              record: null,
              uri: null,
              cid: null,
            },
          });
          return;
        }
        throw error;
      }
    } catch (error: any) {
      set({
        error: error?.message ?? "Failed to get content metadata",
      });
    }
  },

  clearError: () => {
    set({ error: null });
  },
});
