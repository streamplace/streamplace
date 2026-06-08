// Per-livestream content metadata (content warnings, distribution
// policy, content rights). Ported from
// js/app/store/slices/contentMetadataSlice.ts. The slice reads PDS
// agents and the active session from the bluesky slice via get().
// `resolveDIDDocument` / `getPDSServiceEndpoint` are imported from
// the web's lib/did helper (a copy of @streamplace/components/utils/did
// because that package is React-Native-coupled).
import { StateCreator } from "zustand";
import { getPDSServiceEndpoint, resolveDIDDocument } from "../../did";
import { AppStore } from "../index";
import { BlueskySlice } from "./blueskySlice";

export interface ContentMetadataSlice {
  creating: boolean;
  updating: boolean;
  error: string | null;
  lastCreatedRecord: any | null;
  // actions
  createContentMetadata: (params: {
    contentWarnings?: string[];
    distributionPolicy?: { deleteAfter?: number };
    contentRights?: {
      creator?: string;
      copyrightNotice?: string;
      copyrightYear?: number;
      license?: string;
      creditLine?: string;
    };
  }) => Promise<void>;
  updateContentMetadata: (params: {
    rkey?: string;
    livestreamRef?: { uri: string; cid: string };
    contentWarnings?: string[];
    distributionPolicy?: { deleteAfter?: number };
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
    contentWarnings = [],
    distributionPolicy = { deleteAfter: undefined },
    contentRights = {},
  }) => {
    set({ creating: true, error: null });
    try {
      // need access to bluesky slice - combined store provides it
      const state = get() as any;
      const bluesky: BlueskySlice = state;

      if (!bluesky.pdsAgent) {
        throw new Error("No agent");
      }

      const did = bluesky.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }

      const metadataRecord = {
        $type: "place.stream.metadata.configuration",
        createdAt: new Date().toISOString(),
        ...(contentWarnings.length > 0 && {
          contentWarnings: { warnings: contentWarnings },
        }),
        ...(distributionPolicy.deleteAfter && { distributionPolicy }),
        ...(contentRights &&
          Object.keys(contentRights).length > 0 && {
            contentRights,
          }),
      };

      const result = await bluesky.pdsAgent.com.atproto.repo.createRecord({
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
    }
  },

  updateContentMetadata: async ({
    rkey,
    livestreamRef,
    contentWarnings = [],
    distributionPolicy = { deleteAfter: undefined },
    contentRights = {},
  }) => {
    set({ updating: true, error: null });
    try {
      const state = get() as any;
      const bluesky: BlueskySlice = state;

      if (!bluesky.pdsAgent) {
        throw new Error("No agent");
      }

      const did = bluesky.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }

      const metadataRecord = {
        $type: "place.stream.metadata.configuration",
        ...(livestreamRef && { livestreamRef }),
        createdAt: new Date().toISOString(),
        ...(contentWarnings.length > 0 && {
          contentWarnings: { warnings: contentWarnings },
        }),
        ...(distributionPolicy.deleteAfter && { distributionPolicy }),
        ...(contentRights &&
          Object.keys(contentRights).length > 0 && {
            contentRights,
          }),
      };

      const result = await bluesky.pdsAgent.com.atproto.repo.putRecord({
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
    }
  },

  getContentMetadata: async ({ userDid, rkey = "self" } = {}) => {
    set({ error: null });
    try {
      const state = get() as any;
      const bluesky: BlueskySlice = state;

      if (!bluesky.pdsAgent) {
        throw new Error("No agent");
      }

      const targetDid = userDid || bluesky.oauthSession?.did;
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

        let agent = bluesky.pdsAgent;
        if (targetPDS && targetPDS !== (bluesky.pdsAgent as any)?.host) {
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
