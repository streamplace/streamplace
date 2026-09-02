// React bindings for the vanilla LivestreamStore in @streamplace/core.
// useLivestreamStore subscribes a component to a slice of state and
// re-renders on changes. useLivestreamStoreOptional never throws when
// there's no provider above (useful for mode-generic hooks like useTitle).
import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamState,
  type LivestreamStore,
} from "@streamplace/core";
import { useContext } from "react";
import { useStore } from "zustand";
import { LivestreamContext } from "./context";

export { LivestreamStore, makeLivestreamStore };

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

// A shared, never-populated store used as a fallback so the optional hook
// below can call useStore unconditionally even when no LivestreamProvider is
// mounted (e.g. when the player is in "vod" mode).
const emptyLivestreamStore = makeLivestreamStore();

// Like useLivestreamStore, but reads against an empty store instead of
// throwing when there's no LivestreamProvider in the tree. Useful for
// mode-generic hooks (see useTitle) that may run outside a livestream context.
export function useLivestreamStoreOptional<U>(
  selector: (state: LivestreamState) => U,
): U {
  const context = useContext(LivestreamContext);
  const store = context?.store ?? emptyLivestreamStore;
  return useStore(store, selector);
}

export const useHandleWebsocketMessages = () => {
  const store = getStoreFromContext();
  return (messages: any[]) => {
    store.setState((state) => handleWebSocketMessages(state, messages));
  };
};

export const useChat = () => useLivestreamStore((x) => x.chat);

export const usePinnedComment = () =>
  useLivestreamStore((x) => x.pinnedComment);

export const useProfile = () => useLivestreamStore((x) => x.profile);

export const useViewers = () => useLivestreamStore((x) => x.viewers);

export const useLivestream = (includeEnded: boolean = false) =>
  useLivestreamStore((x) => {
    const ls = x.livestream;
    if (!ls) return null;
    if (!includeEnded && ls.record.endedAt !== undefined) return null;
    return ls;
  });

export const useSegment = () => useLivestreamStore((x) => x.segment);

export const useRecentSegments = () =>
  useLivestreamStore((x) => x.recentSegments);

export const useRenditions = () => useLivestreamStore((x) => x.renditions);
