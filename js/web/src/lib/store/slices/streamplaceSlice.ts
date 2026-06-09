// Streamplace server URL, mute flag, chat-warning state, live-users list.
// Mirrors js/app/store/slices/streamplaceSlice.ts.
import type { PlaceStreamLivestream, PlaceStreamSegment } from "streamplace";
import { StateCreator } from "zustand";
import { storage } from "../../storage";
import { getStreamplaceUrl } from "../../streamplace-url";

// Build-time env override. Empty string when not set; the slice falls back
// to `getStreamplaceUrl()` (env > window.location.origin) at runtime.
const ENV_URL = (import.meta.env["VITE_STREAMPLACE_URL"] as string) ?? "";

let DEFAULT_URL = ENV_URL;
if (
  typeof window !== "undefined" &&
  import.meta.env["VITE_WEB_TRY_LOCAL"] === "true"
) {
  try {
    DEFAULT_URL = `${window.location.protocol}//${window.location.host}`;
  } catch {
    // fall back to env
  }
}

export { DEFAULT_URL };

const USER_MUTED_KEY = "streamplaceUserMuted";
const URL_KEY = "streamplaceUrl";
const CHAT_WARNING_KEY = "streamplaceChatWarning2";

export interface Identity {
  id: string;
  handle?: string;
  did?: string;
}

export interface StreamplaceSlice {
  url: string;
  identity: Identity | null;
  initialized: boolean;
  userMuted: boolean | null;
  chatWarned: boolean;
  mySegments: PlaceStreamSegment.SegmentView[];
  liveUsers: PlaceStreamLivestream.LivestreamView[] | null;
  liveUsersLoading: boolean;
  liveUsersError: string | null;
  // actions
  initialize: () => Promise<void>;
  setURL: (url: string) => void;
  userMute: (muted: boolean) => void;
  chatWarn: (warned: boolean) => void;
  getIdentity: () => Promise<void>;
  fetchLiveUsers: () => Promise<void>;
  pollMySegments: () => Promise<void>;
  getRecommendations: (userDID: string) => Promise<{
    recommendations: Array<{
      $type: string;
      did?: string;
      source?: string;
      uri?: string;
    }>;
    userDID?: string;
  }>;
}

export const createStreamplaceSlice: StateCreator<StreamplaceSlice> = (
  set,
  get,
) => ({
  // Initial value resolves at module load. `initialize()` re-reads from
  // storage and may overwrite this with the persisted override.
  url: (() => {
    try {
      return getStreamplaceUrl();
    } catch {
      return DEFAULT_URL;
    }
  })(),
  identity: null,
  initialized: false,
  userMuted: null,
  chatWarned: false,
  mySegments: [],
  liveUsers: null,
  liveUsersLoading: false,
  liveUsersError: null,
  initialize: async () => {
    let [url, userMutedStr, chatWarningStr] = await Promise.all([
      storage.getItem(URL_KEY),
      storage.getItem(USER_MUTED_KEY),
      storage.getItem(CHAT_WARNING_KEY),
    ]);
    if (!url) {
      try {
        url = getStreamplaceUrl();
      } catch {
        url = DEFAULT_URL;
      }
    }
    let userMuted: boolean | null = null;
    if (typeof userMutedStr === "string") {
      userMuted = userMutedStr === "true";
    } else {
      userMuted = null;
    }
    let chatWarned: boolean = false;
    if (typeof chatWarningStr === "string") {
      chatWarned = chatWarningStr === "true";
    }
    set({ url, userMuted, chatWarned, initialized: true });
  },
  setURL: (url: string) => {
    storage.setItem(URL_KEY, url).catch((err) => {
      console.error("setURL error", err);
    });
    set({ url });
  },
  userMute: (muted: boolean) => {
    storage.setItem(USER_MUTED_KEY, JSON.stringify(muted)).catch((err) => {
      console.error("userMute error", err);
    });
    set({ userMuted: muted });
  },
  chatWarn: (warned: boolean) => {
    storage.setItem(CHAT_WARNING_KEY, JSON.stringify(warned)).catch((err) => {
      console.error("chatWarn error", err);
    });
    set({ chatWarned: warned });
  },
  getIdentity: async () => {
    const state = get() as StreamplaceSlice;
    const res = await fetch(`${state.url}/api/identity`);
    const identity = await res.json();
    set({ identity });
  },
  fetchLiveUsers: async () => {
    set({ liveUsersLoading: true, liveUsersError: null });
    try {
      const state = get() as any;
      // anonPDSAgent is created by loadOAuthClient — works even when
      // not logged in. Falls back to constructing one from url if not
      // yet available.
      let agent = state.anonPDSAgent;
      if (!agent) {
        const { StreamplaceAgent } = await import("streamplace");
        agent = new StreamplaceAgent(state.url);
      }
      const result = await agent.place.stream.live.getLiveUsers();
      set({
        liveUsers: result.data.streams ?? [],
        liveUsersLoading: false,
        liveUsersError: null,
      });
    } catch (err: any) {
      set({
        liveUsersLoading: false,
        liveUsersError: err?.message ?? "Failed to fetch live users",
      });
    }
  },
  pollMySegments: async () => {
    try {
      // need access to bluesky slice - will handle in combined store
      const state = get() as any;
      if (!state.pdsAgent) {
        return;
      }
      if (!state.oauthSession) {
        return;
      }
      const result = await state.pdsAgent.place.stream.live.getSegments({
        userDID: state.oauthSession?.did ?? "",
      });
      set({ mySegments: result.data.segments ?? [] });
    } catch {
      // silently fail
    }
  },
  getRecommendations: async (userDID: string) => {
    // need access to bluesky slice - will handle in combined store
    const state = get() as any;
    if (!state.pdsAgent) {
      throw new Error("no pdsAgent");
    }
    const result = await state.pdsAgent.place.stream.live.getRecommendations({
      userDID,
    });
    return result.data;
  },
});
