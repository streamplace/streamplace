import type { CaptionPolicy } from "@streamplace/core";
import type { place } from "streamplace";
import { StateCreator } from "zustand";
import { getPDSServiceEndpoint, resolveDIDDocument } from "../../did";
import { AppStore } from "../index";

export interface ContentMetadataSlice {
  creating: boolean;
  updating: boolean;
  error: string | null;
  lastCreatedRecord: any | null;
  // actions
  createContentMetadata: (params: {
    captionPolicy?: CaptionPolicy;
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
  updateContentMetadata: (params: {
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
  params: Parameters<ContentMetadataSlice["updateContentMetadata"]>[0],
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
  creating: false,
  updating: false,
  error: null,
  lastCreatedRecord: null,

  createContentMetadata: async ({
    contentWarnings,
    distributionPolicy,
    contentRights,
    captionPolicy,
  }) => {
    set({ creating: true, error: null });
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
        contentWarnings,
        distributionPolicy,
        contentRights,
        captionPolicy,
      });

      const result = await pdsAgent.com.atproto.repo.putRecord({
        repo: did,
        collection: "place.stream.metadata.configuration",
        rkey: "self",
        record: metadataRecord,
      });

      const rkey = result.data.uri.split("/").pop();

      set({
        creating: false,
        error: null,
        lastCreatedRecord: {
          record: metadataRecord,
          uri: result.data.uri,
          cid: result.data.cid,
          rkey,
        },
      });
    } catch (error: any) {
      set({
        creating: false,
        error: error?.message ?? "Failed to create content metadata",
      });
      throw error;
    }
  },

  updateContentMetadata: async ({
    rkey,
    livestreamRef,
    contentWarnings,
    distributionPolicy,
    contentRights,
    captionPolicy,
  }) => {
    set({ updating: true, error: null });
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
        rkey: rkey || "self",
        record: metadataRecord,
      });

      set({
        updating: false,
        error: null,
        lastCreatedRecord: {
          record: metadataRecord,
          uri: `at://${did}/place.stream.metadata.configuration/${
            rkey || "self"
          }`,
          cid: result.data.cid,
        },
      });
    } catch (error: any) {
      set({
        updating: false,
        error: error?.message ?? "Failed to update content metadata",
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
