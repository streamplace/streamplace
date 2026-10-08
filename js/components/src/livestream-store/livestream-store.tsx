import { useContext } from "react";
import { createStore, StoreApi, useStore } from "zustand";
import { LivestreamContext } from "./context";
import { LivestreamState } from "./livestream-state";
import { handleWebSocketMessages } from "./websocket-consumer";

export type LivestreamStore = StoreApi<LivestreamState>;

export const makeLivestreamStore = (): StoreApi<LivestreamState> => {
  return createStore<LivestreamState>()((set) => ({
    profile: null,
    chatIndex: {},
    chat: [],
    livestream: null,
    viewers: null,
    pendingHides: [],
    segment: null,
    renditions: [],
    replyToMessage: null,
    streamKey: null,
    setStreamKey: (sk) => set({ streamKey: sk }),
    authors: {},
    recentSegments: [],
    problems: [],
    activeTeleport: null,
    activeTeleportUri: null,
    setActiveTeleportUri: (uri) => set({ activeTeleportUri: uri }),
    websocketConnected: false,
    hasReceivedSegment: false,
    pinnedComment: null,
    moderationPermissions: [],
    setModerationPermissions: (perms) => set({ moderationPermissions: perms }),
    localLivestreamURI: null,
    setLocalLivestreamURI: (uri) => set({ localLivestreamURI: uri }),
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

/**
 * Wall-clock time the current livestream started, in epoch milliseconds —
 * the anchor that makes temporal references (#t=…) point at a fixed moment.
 * Null when there is no live livestream record to anchor to.
 */
export const useStreamStartMs = (): number | null => {
  const livestream = useLivestream();
  if (!livestream) {
    return null;
  }
  const ms = Date.parse(livestream.record.createdAt);
  return Number.isNaN(ms) ? null : ms;
};

export const useRecentSegments = () =>
  useLivestreamStore((x) => x.recentSegments);

export const useRenditions = () => useLivestreamStore((x) => x.renditions);
