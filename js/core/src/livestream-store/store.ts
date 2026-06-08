// Vanilla Zustand store for the livestream state. No React imports.
//
// The React bindings (`useLivestreamStore`, `useLivestreamStoreOptional`,
// `getStoreFromContext`, etc.) live in @streamplace/components.
import { createStore, StoreApi } from "zustand";
import { LivestreamState } from "./state";

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
    chatDraft: "",
    badgeSlots: null,
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
