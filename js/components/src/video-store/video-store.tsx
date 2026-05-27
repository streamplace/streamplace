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

/* Convenience selectors/hooks */
export const useVideo = () => useVideoStore((x) => x.video);

export const useVideoLoading = () => useVideoStore((x) => x.loading);

export const useVideoError = () => useVideoStore((x) => x.error);
