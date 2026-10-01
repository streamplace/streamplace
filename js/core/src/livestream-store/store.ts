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
    viewTotal: null,
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
    deletedModerationPermissionURIs: [],
    moderationPermissionRevisions: {},
    setModerationPermissions: (permissions) =>
      set((state) => {
        const deletedURIs = new Set(state.deletedModerationPermissionURIs);
        return {
          moderationPermissions: permissions.filter(
            (permission) => !permission.uri || !deletedURIs.has(permission.uri),
          ),
        };
      }),
    localLivestreamURI: null,
    setLocalLivestreamURI: (uri) => set({ localLivestreamURI: uri }),
    captionTracks: [],
    liveCaptions: {},
  }));
};
