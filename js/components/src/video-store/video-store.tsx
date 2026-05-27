import { useContext } from "react";
import { createStore, StoreApi, useStore } from "zustand";
import { VideoContext } from "./context";
import { VideoState } from "./video-state";

export type VideoStore = StoreApi<VideoState>;

export const makeVideoStore = ({ aturi }: { aturi: string }): VideoStore => {
  return createStore<VideoState>()((set) => ({
    aturi,
    setAturi: (aturi) => set({ aturi }),
    video: null,
    setVideo: (video) => set({ video }),
    loading: false,
    setLoading: (loading) => set({ loading }),
    error: null,
    setError: (error) => set({ error }),
  }));
};

export function getVideoStoreFromContext(): VideoStore {
  const context = useContext(VideoContext);
  if (!context) {
    throw new Error("useVideoStore must be used within a VideoProvider");
  }
  return context.store;
}

export function useVideoStore<U>(selector: (state: VideoState) => U): U {
  const store = getVideoStoreFromContext();
  return useStore(store, selector);
}

// A shared, never-populated store used as a fallback so the optional hook
// below can call useStore unconditionally even when no VideoProvider is
// mounted (e.g. when the player is in "live" mode).
const emptyVideoStore = makeVideoStore({ aturi: "" });

// Like useVideoStore, but reads against an empty store instead of throwing
// when there's no VideoProvider in the tree. Useful for mode-generic hooks
// (see useTitle) that may run outside a video context.
export function useVideoStoreOptional<U>(
  selector: (state: VideoState) => U,
): U {
  const context = useContext(VideoContext);
  const store = context?.store ?? emptyVideoStore;
  return useStore(store, selector);
}

/* Convenience selectors/hooks */
export const useVideo = () => useVideoStore((x) => x.video);

export const useVideoLoading = () => useVideoStore((x) => x.loading);

export const useVideoError = () => useVideoStore((x) => x.error);
