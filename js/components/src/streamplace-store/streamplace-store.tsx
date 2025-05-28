import { useContext } from "react";
import { PlaceStreamLivestream } from "streamplace";
import { createStore, StoreApi, useStore } from "zustand";
import { StreamplaceContext } from "../streamplace-provider/context";

export interface StreamplaceState {
  url: string;
  liveUsers: PlaceStreamLivestream.LivestreamView[];
  setLiveUsers: (users: PlaceStreamLivestream.LivestreamView[]) => void;
}

export type StreamplaceStore = StoreApi<StreamplaceState>;

export const makeStreamplaceStore = ({
  url,
}: {
  url: string;
}): StoreApi<StreamplaceState> => {
  return createStore<StreamplaceState>()((set) => ({
    url,
    liveUsers: [],
    setLiveUsers: (users: PlaceStreamLivestream.LivestreamView[]) => {
      set({ liveUsers: users });
    },
  }));
};

export function useStreamplaceStore<U>(
  selector: (state: StreamplaceState) => U,
): U {
  const context = useContext(StreamplaceContext);
  if (!context) {
    throw new Error(
      "useStreamplaceStore must be used within a StreamplaceProvider",
    );
  }
  return useStore(context.store, selector);
}

export const useUrl = () => useStreamplaceStore((x) => x.url);
