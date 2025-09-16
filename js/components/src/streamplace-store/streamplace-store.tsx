import { SessionManager } from "@atproto/api/dist/session-manager";
import { useContext } from "react";
import {
  PlaceStreamChatProfile,
  PlaceStreamLivestream,
  PlaceStreamMetadataConfiguration,
} from "streamplace";
import { createStore, StoreApi, useStore } from "zustand";
import storage from "../storage";
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
  oauthSession: SessionManager | null | undefined;
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

  // Volume state
  volume: number;
  muted: boolean;
  setVolume: (volume: number) => void;
  setMuted: (muted: boolean) => void;
}

export type StreamplaceStore = StoreApi<StreamplaceState>;

export const makeStreamplaceStore = ({
  url,
}: {
  url: string;
}): StoreApi<StreamplaceState> => {
  const VOLUME_STORAGE_KEY = "globalVolume";
  const MUTED_STORAGE_KEY = "globalMuted";

  const store = createStore<StreamplaceState>()((set) => ({
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

    // Volume state - start with defaults
    volume: 1.0,
    muted: false,

    setVolume: (volume: number) => {
      // Ensure the value is finite and within bounds
      if (!Number.isFinite(volume)) {
        console.warn("Invalid volume value:", volume, "- using 1.0");
        volume = 1.0;
      }
      const clampedVolume = Math.max(0, Math.min(1, volume));

      set({ volume: clampedVolume });

      // Auto-unmute if volume > 0
      if (clampedVolume > 0) {
        set({ muted: false });
        storage.setItem(MUTED_STORAGE_KEY, "false").catch(console.error);
      }

      storage
        .setItem(VOLUME_STORAGE_KEY, clampedVolume.toString())
        .catch(console.error);
    },

    setMuted: (muted: boolean) => {
      set({ muted });
      storage.setItem(MUTED_STORAGE_KEY, muted.toString()).catch(console.error);
    },
  }));

  // Load initial volume state from storage asynchronously
  (async () => {
    try {
      const storedVolume = await storage.getItem(VOLUME_STORAGE_KEY);
      const storedMuted = await storage.getItem(MUTED_STORAGE_KEY);

      let initialVolume = 1.0;
      let initialMuted = false;

      if (storedVolume) {
        const parsedVolume = parseFloat(storedVolume);
        if (
          Number.isFinite(parsedVolume) &&
          parsedVolume >= 0 &&
          parsedVolume <= 1
        ) {
          initialVolume = parsedVolume;
        }
      }

      if (storedMuted) {
        initialMuted = storedMuted === "true";
      }

      // Update the store with loaded values
      store.setState({
        volume: initialVolume,
        muted: initialMuted,
      });
    } catch (e) {
      console.warn("Failed to load volume settings from storage:", e);
    }
  })();

  return store;
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

// Volume convenience hooks
export const useVolume = () => useStreamplaceStore((x) => x.volume);
export const useMuted = () => useStreamplaceStore((x) => x.muted);
export const useSetVolume = () => useStreamplaceStore((x) => x.setVolume);
export const useSetMuted = () => useStreamplaceStore((x) => x.setMuted);

// Composite hook for effective volume (0 if muted) - used by video components
export const useEffectiveVolume = () =>
  useStreamplaceStore((state) => {
    const effectiveVolume = state.muted ? 0 : state.volume;
    // Ensure we always return a finite number for HTMLMediaElement.volume
    return Number.isFinite(effectiveVolume) ? effectiveVolume : 1.0;
  });
