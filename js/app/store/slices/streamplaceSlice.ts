import { storage } from "@streamplace/components";
import { Platform } from "react-native";
import type { PlaceStreamSegment } from "streamplace";
import { StateCreator } from "zustand";

let DEFAULT_URL = process.env.EXPO_PUBLIC_STREAMPLACE_URL as string;
if (Platform.OS === "web" && process.env.EXPO_PUBLIC_WEB_TRY_LOCAL === "true") {
  try {
    DEFAULT_URL = `${window.location.protocol}//${window.location.host}`;
  } catch (err) {
    // fall back to hardcoded
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
  // actions
  initialize: () => Promise<void>;
  setURL: (url: string) => void;
  userMute: (muted: boolean) => void;
  chatWarn: (warned: boolean) => void;
  getIdentity: () => Promise<void>;
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
  url: DEFAULT_URL,
  identity: null,
  initialized: false,
  userMuted: null,
  chatWarned: false,
  mySegments: [],
  initialize: async () => {
    let [url, userMutedStr, chatWarningStr] = await Promise.all([
      storage.getItem(URL_KEY),
      storage.getItem(USER_MUTED_KEY),
      storage.getItem(CHAT_WARNING_KEY),
    ]);
    if (!url) {
      url = DEFAULT_URL;
    }
    let userMuted: boolean | null = null;
    console.log("userMutedStr", userMutedStr);
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
    console.log("setURL", url);
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
  pollMySegments: async () => {
    try {
      const state = get() as any; // need to access bluesky slice
      if (!state.pdsAgent) {
        throw new Error("no pdsAgent");
      }
      if (!state.oauthSession) {
        throw new Error("no oauthSession");
      }
      const result = await state.pdsAgent.place.stream.live.getSegments({
        userDID: state.oauthSession?.did ?? "",
      });
      set({ mySegments: result.data.segments ?? [] });
    } catch (err) {
      // silently fail
    }
  },
  getRecommendations: async (userDID: string) => {
    const state = get() as any; // need to access bluesky slice
    if (!state.pdsAgent) {
      throw new Error("no pdsAgent");
    }
    const result = await state.pdsAgent.place.stream.live.getRecommendations({
      userDID,
    });
    return result.data;
  },
});
