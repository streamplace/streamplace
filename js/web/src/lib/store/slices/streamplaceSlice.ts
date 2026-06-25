import { place } from "streamplace";
// Streamplace server URL, mute flag, chat-warning state.
//
// Live-users polling now lives in hooks/use-live-users.ts (React Query).
import { StateCreator } from "zustand";
import { storage } from "../../storage";
import { getStreamplaceUrl } from "../../streamplace-url";
import { AppStore } from "../index";

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

export interface StreamplaceSlice {
  url: string;
  initialized: boolean;
  userMuted: boolean | null;
  chatWarned: boolean;
  // actions
  initialize: () => Promise<void>;
  setURL: (url: string) => void;
  userMute: (muted: boolean) => void;
  chatWarn: (warned: boolean) => void;
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

export const createStreamplaceSlice: StateCreator<
  AppStore,
  [],
  [],
  StreamplaceSlice
> = (set, get) => ({
  // Initial value resolves at module load. `initialize()` re-reads from
  // storage and may overwrite this with the persisted override.
  url: (() => {
    try {
      return getStreamplaceUrl();
    } catch {
      return DEFAULT_URL;
    }
  })(),
  initialized: false,
  userMuted: null,
  chatWarned: false,
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
  getRecommendations: async (userDID: string) => {
    // Fall back to the anonymous PDS agent when the user isn't logged
    // in. The getRecommendations endpoint is a public read on the
    // Streamplace server; it returns the streamer's own
    // recommendations list filtered to currently-live streamers, or
    // falls back to the streamer's follows that are live. No auth
    // required, so a logged-out viewer can still see suggestions.
    const { pdsAgent, anonPDSAgent } = get();
    const agent = pdsAgent ?? anonPDSAgent;
    if (!agent) {
      throw new Error("no pdsAgent");
    }
    const result = await agent.client.call(place.stream.live.getRecommendations, {
      userDID: userDID as any,
    });
    return result;
  },
});
