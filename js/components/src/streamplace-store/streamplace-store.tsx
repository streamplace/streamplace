import { SessionManager } from "@atproto/api/dist/session-manager";
import { useContext } from "react";
import { PlaceStreamChatProfile, PlaceStreamLivestream } from "streamplace";
import { createStore, StoreApi, useStore } from "zustand";
import storage from "../storage";
import { StreamplaceContext } from "../streamplace-provider/context";

export interface ContentMetadataResult {
  record: any;
  uri: string;
  cid: string;
  rkey?: string;
}

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
  liveUsers: PlaceStreamLivestream.LivestreamView[] | null;
  setLiveUsers: (opts: {
    liveUsers?: PlaceStreamLivestream.LivestreamView[];
    liveUsersLoading?: boolean;
    liveUsersError?: string | null;
    liveUsersRefresh?: number;
  }) => void;
  liveUsersRefresh: number;
  liveUsersLoading: boolean;
  liveUsersError: string | null;
  oauthSession: SessionManager | null | undefined;
  handle: string | null;
  chatProfile: PlaceStreamChatProfile.Record | null;

  // Content metadata state
  contentMetadata: ContentMetadataResult | null;
  setContentMetadata: (metadata: ContentMetadataResult | null) => void;

  broadcasterDID: string | null;
  setBroadcasterDID: (broadcasterDID: string | null) => void;
  serverDID: string | null;
  setServerDID: (serverDID: string | null) => void;

  // Volume state
  volume: number;
  muted: boolean;
  setVolume: (volume: number) => void;
  setMuted: (muted: boolean) => void;

  // Danmu settings
  danmuUnlocked: boolean;
  danmuEnabled: boolean;
  danmuOpacity: number;
  danmuSpeed: number;
  danmuLaneCount: number;
  danmuMaxMessages: number;
  setDanmuUnlocked: (unlocked: boolean) => void;
  setDanmuEnabled: (enabled: boolean) => void;
  setDanmuOpacity: (opacity: number) => void;
  setDanmuSpeed: (speed: number) => void;
  setDanmuLaneCount: (laneCount: number) => void;
  setDanmuMaxMessages: (maxMessages: number) => void;
}

export type StreamplaceStore = StoreApi<StreamplaceState>;

export const makeStreamplaceStore = ({
  url,
}: {
  url: string;
}): StoreApi<StreamplaceState> => {
  const VOLUME_STORAGE_KEY = "globalVolume";
  const MUTED_STORAGE_KEY = "globalMuted";
  const DANMU_UNLOCKED_KEY = "danmuUnlocked";
  const DANMU_ENABLED_KEY = "danmuEnabled";
  const DANMU_OPACITY_KEY = "danmuOpacity";
  const DANMU_SPEED_KEY = "danmuSpeed";
  const DANMU_LANE_COUNT_KEY = "danmuLaneCount";
  const DANMU_MAX_MESSAGES_KEY = "danmuMaxMessages";

  const store = createStore<StreamplaceState>()((set) => ({
    url,
    liveUsers: null,
    setLiveUsers: (opts: {
      liveUsers?: PlaceStreamLivestream.LivestreamView[];
      liveUsersLoading?: boolean;
      liveUsersError?: string | null;
      liveUsersRefresh?: number;
    }) => {
      set({
        ...opts,
      });
    },
    liveUsersRefresh: 0,
    liveUsersLoading: true,
    liveUsersError: null,
    oauthSession: null,
    handle: null,
    chatProfile: null,

    broadcasterDID: null,
    setBroadcasterDID: (broadcasterDID: string | null) =>
      set({ broadcasterDID }),
    serverDID: null,
    setServerDID: (serverDID: string | null) => set({ serverDID }),

    // Content metadata
    contentMetadata: null,
    setContentMetadata: (metadata) => set({ contentMetadata: metadata }),

    // Volume state - start with defaults
    volume: 1.0,
    muted: false,

    setVolume: (volume: number) => {
      // Ensure the value is finite and within bounds
      if (!Number.isFinite(volume)) {
        console.warn("Invalid volume value:", volume, "- using 1.0");
        volume = 1.0;
      }
      const clampedVolume = Math.max(0, Math.min(1, volume));

      set({ volume: clampedVolume });

      // Auto-unmute if volume > 0
      if (clampedVolume > 0) {
        set({ muted: false });
        storage.setItem(MUTED_STORAGE_KEY, "false").catch(console.error);
      }

      storage
        .setItem(VOLUME_STORAGE_KEY, clampedVolume.toString())
        .catch(console.error);
    },

    setMuted: (muted: boolean) => {
      set({ muted });
      storage.setItem(MUTED_STORAGE_KEY, muted.toString()).catch(console.error);
    },

    // Danmu settings - start with defaults
    danmuUnlocked: false,
    danmuEnabled: false,
    danmuOpacity: 80,
    danmuSpeed: 1,
    danmuLaneCount: 12,
    danmuMaxMessages: 50,

    setDanmuUnlocked: (unlocked: boolean) => {
      set({ danmuUnlocked: unlocked });
      storage
        .setItem(DANMU_UNLOCKED_KEY, unlocked.toString())
        .catch(console.error);
    },

    setDanmuEnabled: (enabled: boolean) => {
      set({ danmuEnabled: enabled });
      storage
        .setItem(DANMU_ENABLED_KEY, enabled.toString())
        .catch(console.error);
    },

    setDanmuOpacity: (opacity: number) => {
      const clamped = Math.max(0, Math.min(100, opacity));
      set({ danmuOpacity: clamped });
      storage
        .setItem(DANMU_OPACITY_KEY, clamped.toString())
        .catch(console.error);
    },

    setDanmuSpeed: (speed: number) => {
      const clamped = Math.max(0.1, Math.min(3, speed));
      set({ danmuSpeed: clamped });
      storage.setItem(DANMU_SPEED_KEY, clamped.toString()).catch(console.error);
    },

    setDanmuLaneCount: (laneCount: number) => {
      const clamped = Math.max(4, Math.min(20, laneCount));
      set({ danmuLaneCount: clamped });
      storage
        .setItem(DANMU_LANE_COUNT_KEY, clamped.toString())
        .catch(console.error);
    },

    setDanmuMaxMessages: (maxMessages: number) => {
      const clamped = Math.max(5, Math.min(200, maxMessages));
      set({ danmuMaxMessages: clamped });
      storage
        .setItem(DANMU_MAX_MESSAGES_KEY, clamped.toString())
        .catch(console.error);
    },
  }));

  // Load initial volume and danmu state from storage asynchronously
  (async () => {
    try {
      const storedVolume = await storage.getItem(VOLUME_STORAGE_KEY);
      const storedMuted = await storage.getItem(MUTED_STORAGE_KEY);
      const storedDanmuUnlocked = await storage.getItem(DANMU_UNLOCKED_KEY);
      const storedDanmuEnabled = await storage.getItem(DANMU_ENABLED_KEY);
      const storedDanmuOpacity = await storage.getItem(DANMU_OPACITY_KEY);
      const storedDanmuSpeed = await storage.getItem(DANMU_SPEED_KEY);
      const storedDanmuLaneCount = await storage.getItem(DANMU_LANE_COUNT_KEY);
      const storedDanmuMaxMessages = await storage.getItem(
        DANMU_MAX_MESSAGES_KEY,
      );

      let initialVolume = 1.0;
      let initialMuted = false;
      let initialDanmuUnlocked = false;
      let initialDanmuEnabled = false;
      let initialDanmuOpacity = 80;
      let initialDanmuSpeed = 1;
      let initialDanmuLaneCount = 12;
      let initialDanmuMaxMessages = 50;

      if (storedVolume) {
        const parsedVolume = parseFloat(storedVolume);
        if (
          Number.isFinite(parsedVolume) &&
          parsedVolume >= 0 &&
          parsedVolume <= 1
        ) {
          initialVolume = parsedVolume;
        }
      }

      if (storedMuted) {
        initialMuted = storedMuted === "true";
      }

      if (storedDanmuUnlocked) {
        initialDanmuUnlocked = storedDanmuUnlocked === "true";
      }

      if (storedDanmuEnabled) {
        initialDanmuEnabled = storedDanmuEnabled === "true";
      }

      if (storedDanmuOpacity) {
        const parsed = parseInt(storedDanmuOpacity);
        if (Number.isFinite(parsed) && parsed >= 0 && parsed <= 100) {
          initialDanmuOpacity = parsed;
        }
      }

      if (storedDanmuSpeed) {
        const parsed = parseFloat(storedDanmuSpeed);
        if (Number.isFinite(parsed) && parsed >= 0.1 && parsed <= 3) {
          initialDanmuSpeed = parsed;
        }
      }

      if (storedDanmuLaneCount) {
        const parsed = parseInt(storedDanmuLaneCount);
        if (Number.isFinite(parsed) && parsed >= 4 && parsed <= 20) {
          initialDanmuLaneCount = parsed;
        }
      }

      if (storedDanmuMaxMessages) {
        const parsed = parseInt(storedDanmuMaxMessages);
        if (Number.isFinite(parsed) && parsed >= 5 && parsed <= 200) {
          initialDanmuMaxMessages = parsed;
        }
      }

      store.setState({
        volume: initialVolume,
        muted: initialMuted,
        danmuUnlocked: initialDanmuUnlocked,
        danmuEnabled: initialDanmuEnabled,
        danmuOpacity: initialDanmuOpacity,
        danmuSpeed: initialDanmuSpeed,
        danmuLaneCount: initialDanmuLaneCount,
        danmuMaxMessages: initialDanmuMaxMessages,
      });
    } catch (error) {
      console.error("Failed to load state from storage:", error);
    }
  })();

  return store;
};

export function getStreamplaceStoreFromContext(): StreamplaceStore {
  const context = useContext(StreamplaceContext);
  if (!context) {
    throw new Error(
      "useStreamplaceStore must be used within a StreamplaceProvider",
    );
  }
  return context.store;
}

export function useStreamplaceStore<U>(
  selector: (state: StreamplaceState) => U,
): U {
  return useStore(getStreamplaceStoreFromContext(), selector);
}

export const useUrl = () => useStreamplaceStore((x) => x.url);

export const useDID = () => useStreamplaceStore((x) => x.oauthSession?.did);

export const useHandle = () => useStreamplaceStore((x) => x.handle);
export const useSetHandle = (): ((handle: string) => void) => {
  const store = getStreamplaceStoreFromContext();
  return (handle: string) => store.setState({ handle });
};

// Content metadata hooks
export const useContentMetadata = () =>
  useStreamplaceStore((x) => x.contentMetadata);

export const useSetContentMetadata = () => {
  const store = getStreamplaceStoreFromContext();
  return (metadata: ContentMetadataResult | null) =>
    store.setState({ contentMetadata: metadata });
};

// Volume/muted hooks
export const useVolume = () => useStreamplaceStore((x) => x.volume);
export const useMuted = () => useStreamplaceStore((x) => x.muted);
export const useSetVolume = () => useStreamplaceStore((x) => x.setVolume);
export const useSetMuted = () => useStreamplaceStore((x) => x.setMuted);

// Composite hook for effective volume (0 if muted) - used by video components
export const useEffectiveVolume = () =>
  useStreamplaceStore((state) => {
    const effectiveVolume = state.muted ? 0 : state.volume;
    // Ensure we always return a finite number for HTMLMediaElement.volume
    return Number.isFinite(effectiveVolume) ? effectiveVolume : 1.0;
  });

// Danmu convenience hooks
export const useDanmuUnlocked = () =>
  useStreamplaceStore((x) => x.danmuUnlocked);
export const useDanmuEnabled = () => useStreamplaceStore((x) => x.danmuEnabled);
export const useDanmuOpacity = () => useStreamplaceStore((x) => x.danmuOpacity);
export const useDanmuSpeed = () => useStreamplaceStore((x) => x.danmuSpeed);
export const useDanmuLaneCount = () =>
  useStreamplaceStore((x) => x.danmuLaneCount);
export const useDanmuMaxMessages = () =>
  useStreamplaceStore((x) => x.danmuMaxMessages);
export const useSetDanmuUnlocked = () =>
  useStreamplaceStore((x) => x.setDanmuUnlocked);
export const useSetDanmuEnabled = () =>
  useStreamplaceStore((x) => x.setDanmuEnabled);
export const useSetDanmuOpacity = () =>
  useStreamplaceStore((x) => x.setDanmuOpacity);
export const useSetDanmuSpeed = () =>
  useStreamplaceStore((x) => x.setDanmuSpeed);
export const useSetDanmuLaneCount = () =>
  useStreamplaceStore((x) => x.setDanmuLaneCount);
export const useSetDanmuMaxMessages = () =>
  useStreamplaceStore((x) => x.setDanmuMaxMessages);

// Composite hook that calls all individual hooks
export const useDanmuSettings = () => {
  const danmuUnlocked = useDanmuUnlocked();
  const danmuEnabled = useDanmuEnabled();
  const danmuOpacity = useDanmuOpacity();
  const danmuSpeed = useDanmuSpeed();
  const danmuLaneCount = useDanmuLaneCount();
  const danmuMaxMessages = useDanmuMaxMessages();
  const setDanmuUnlocked = useSetDanmuUnlocked();
  const setDanmuEnabled = useSetDanmuEnabled();
  const setDanmuOpacity = useSetDanmuOpacity();
  const setDanmuSpeed = useSetDanmuSpeed();
  const setDanmuLaneCount = useSetDanmuLaneCount();
  const setDanmuMaxMessages = useSetDanmuMaxMessages();

  return {
    danmuUnlocked,
    danmuEnabled,
    danmuOpacity,
    danmuSpeed,
    danmuLaneCount,
    danmuMaxMessages,
    setDanmuUnlocked,
    setDanmuEnabled,
    setDanmuOpacity,
    setDanmuSpeed,
    setDanmuLaneCount,
    setDanmuMaxMessages,
  };
};

export { useCreateStreamRecord, useUpdateStreamRecord } from "./stream";
