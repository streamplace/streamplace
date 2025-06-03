import { SessionManager } from "@atproto/api/dist/session-manager";
import { useContext } from "react";
import { PlaceStreamLivestream } from "streamplace";
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

export interface StreamplaceState {
  url: string;
  liveUsers: PlaceStreamLivestream.LivestreamView[];
  setLiveUsers: (users: PlaceStreamLivestream.LivestreamView[]) => void;
  oauthSession: SessionManager | null;
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
    oauthSession: null,
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

export const useDID = () => useStreamplaceStore((x) => x.oauthSession?.did);
