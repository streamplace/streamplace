import { useContext } from "react";
import { createStore, StoreApi, useStore } from "zustand";
import { LivestreamContext } from "./context";
import { LivestreamState } from "./livestream-state";
import { handleWebSocketMessages } from "./websocket";

export type LivestreamStore = StoreApi<LivestreamState>;

export const makeLivestreamStore = (): StoreApi<LivestreamState> => {
  return createStore<LivestreamState>()((set) => ({
    profile: null,
    chatIndex: {},
    chat: [],
    handleWebSocketMessages: (messages: any[]) =>
      set((state) => handleWebSocketMessages(state, messages)),
    livestream: null,
    viewers: null,
    segment: null,
    renditions: [],
    replyToMessage: null,
  }));
};

export function getStoreFromContext(): LivestreamStore {
  const context = useContext(LivestreamContext);
  if (!context) {
    throw new Error(
      "useLivestreamStore must be used within a LivestreamProvider",
    );
  }
  return context.store;
}

export function useLivestreamStore<U>(
  selector: (state: LivestreamState) => U,
): U {
  const store = getStoreFromContext();
  return useStore(store, selector);
}

export const useChat = () => useLivestreamStore((x) => x.chat);

export const useHandleWebsocketMessages = () =>
  useLivestreamStore((x) => x.handleWebSocketMessages);

export const useProfile = () => useLivestreamStore((x) => x.profile);

export const useViewers = () => useLivestreamStore((x) => x.viewers);

export const useLivestream = () => useLivestreamStore((x) => x.livestream);

export const useSegment = () => useLivestreamStore((x) => x.segment);
