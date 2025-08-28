import { SessionManager } from "@atproto/api/dist/session-manager";
import { useContext } from "react";
import {
  PlaceStreamChatProfile,
  PlaceStreamLivestream,
  PlaceStreamMetadataConfiguration,
} from "streamplace";
import { createStore, StoreApi, useStore } from "zustand";
import { StreamplaceContext } from "../streamplace-provider/context";

// there are three categories of XRPC that we need to handle:
// 1. Public (probably) OAuth XRPC to the users' PDS for apps that use this API.
// 2. Confidental OAuth to the Streamplace server for doing things that require
//    server-side authentication. This isn't very much stuff yet, but you need
//    to log into Streamplace to do things like have Streamplace update your
//    activity status.
// 3. Anonymous XRPC to the Streamplace server for stuff like `getLiveUsers`. This
//    is easy to handle internal to this library.
// For the Streamplace app itself, all three are the same. For apps that aren't
// doing OAuth through the Streamplace node, we need to expose an interface that
// allows them to use atcute or whatever for 1.

export interface ContentMetadataResult {
  record: PlaceStreamMetadataConfiguration.Record;
  uri: string;
  cid: string;
  rkey?: string;
}

export interface StreamplaceState {
  url: string;
  liveUsers: PlaceStreamLivestream.LivestreamView[] | null;
  setLiveUsers: (opts: {
    liveUsers?: PlaceStreamLivestream.LivestreamView[];
    liveUsersLoading?: boolean;
    liveUsersError?: string | null;
    liveUsersRefresh?: number;
  }) => void;
  liveUsersRefresh: number;
  liveUsersLoading: boolean;
  liveUsersError: string | null;
  oauthSession: SessionManager | null;
  handle: string | null;
  chatProfile: PlaceStreamChatProfile.Record | null;

  // Content metadata state
  contentMetadata: ContentMetadataResult | null;
  contentMetadataCreating: boolean;
  contentMetadataUpdating: boolean;
  contentMetadataError: string | null;
  setContentMetadata: (opts: {
    contentMetadata?: ContentMetadataResult | null;
    creating?: boolean;
    updating?: boolean;
    error?: string | null;
  }) => void;
}

export type StreamplaceStore = StoreApi<StreamplaceState>;

export const makeStreamplaceStore = ({
  url,
}: {
  url: string;
}): StoreApi<StreamplaceState> => {
  return createStore<StreamplaceState>()((set) => ({
    url,
    liveUsers: null,
    setLiveUsers: (opts: {
      liveUsers?: PlaceStreamLivestream.LivestreamView[];
      liveUsersLoading?: boolean;
      liveUsersError?: string | null;
      liveUsersRefresh?: number;
    }) => {
      set({
        ...opts,
      });
    },
    liveUsersRefresh: 0,
    liveUsersLoading: true,
    liveUsersError: null,
    oauthSession: null,
    handle: null,
    chatProfile: null,

    // Content metadata initial state
    contentMetadata: null,
    contentMetadataCreating: false,
    contentMetadataUpdating: false,
    contentMetadataError: null,
    setContentMetadata: (opts: {
      contentMetadata?: ContentMetadataResult | null;
      creating?: boolean;
      updating?: boolean;
      error?: string | null;
    }) => {
      set({
        ...(opts.contentMetadata !== undefined && {
          contentMetadata: opts.contentMetadata,
        }),
        ...(opts.creating !== undefined && {
          contentMetadataCreating: opts.creating,
        }),
        ...(opts.updating !== undefined && {
          contentMetadataUpdating: opts.updating,
        }),
        ...(opts.error !== undefined && { contentMetadataError: opts.error }),
      });
    },
  }));
};

export function getStreamplaceStoreFromContext(): StreamplaceStore {
  const context = useContext(StreamplaceContext);
  if (!context) {
    throw new Error(
      "useStreamplaceStore must be used within a StreamplaceProvider",
    );
  }
  return context.store;
}

export function useStreamplaceStore<U>(
  selector: (state: StreamplaceState) => U,
): U {
  return useStore(getStreamplaceStoreFromContext(), selector);
}

export const useUrl = () => useStreamplaceStore((x) => x.url);

export const useDID = () => useStreamplaceStore((x) => x.oauthSession?.did);

export const useHandle = () => useStreamplaceStore((x) => x.handle);
export const useSetHandle = (): ((handle: string) => void) => {
  const store = getStreamplaceStoreFromContext();
  return (handle: string) => store.setState({ handle });
};

// Content metadata hooks
export const useContentMetadata = () =>
  useStreamplaceStore((x) => x.contentMetadata);
export const useIsCreating = () =>
  useStreamplaceStore((x) => x.contentMetadataCreating);
export const useIsUpdating = () =>
  useStreamplaceStore((x) => x.contentMetadataUpdating);
export const useContentMetadataError = () =>
  useStreamplaceStore((x) => x.contentMetadataError);

export const useSetContentMetadata = () => {
  const store = getStreamplaceStoreFromContext();
  return store.getState().setContentMetadata;
};
