import { ComAtprotoModerationCreateReport } from "@atproto/api";
import { useContext, useEffect, useState } from "react";
import { ChatMessageViewHydrated } from "streamplace";
import { createStore, StoreApi, useStore } from "zustand";
import { useLivestreamStore } from "../livestream-store";
import storage from "../storage";
import { useStreamplaceStore } from "../streamplace-store";
import { PlayerContext } from "./context";
import {
  IngestMediaSource,
  PlayerEvent,
  PlayerProtocol,
  PlayerState,
  PlayerStatus,
} from "./player-state";

const PROTOCOL_STORAGE_KEY = "player-protocol";

export type PlayerStore = StoreApi<PlayerState>;

export const makePlayerStore = (id?: string): StoreApi<PlayerState> => {
  const store = createStore<PlayerState>()((set) => ({
    id: id || Math.random().toString(36).slice(8),
    mode: "live",
    setMode: (mode) => set(() => ({ mode })),
    duration: 0,
    setDuration: (duration) => set(() => ({ duration })),
    bufferedEnd: 0,
    setBufferedEnd: (bufferedEnd) => set(() => ({ bufferedEnd })),
    vodLevels: [],
    setVodLevels: (vodLevels) => set(() => ({ vodLevels })),
    playingVODRendition: null,
    setPlayingVODRendition: (playingVODRendition) =>
      set(() => ({ playingVODRendition })),
    playingLiveRendition: null,
    setPlayingLiveRendition: (playingLiveRendition) =>
      set(() => ({ playingLiveRendition })),

    selectedRendition: "source",
    setSelectedRendition: (rendition: string) =>
      set((state) => {
        if (rendition === "audio" && state.controlsTimeout) {
          clearTimeout(state.controlsTimeout);
          return {
            ...state,
            selectedRendition: rendition,
            showControls: true,
            controlsTimeout: undefined,
          };
        }
        return { ...state, selectedRendition: rendition };
      }),
    protocol: PlayerProtocol.WEBRTC,
    setProtocol: (protocol: PlayerProtocol) => {
      storage.setItem(PROTOCOL_STORAGE_KEY, protocol).catch(console.error);
      set((state) => ({ ...state, protocol: protocol }));
    },

    src: "",
    setSrc: (src: string) => set(() => ({ src })),

    ingestMediaSource: undefined,
    setIngestMediaSource: (ingestMediaSource: IngestMediaSource | undefined) =>
      set(() => ({ ingestMediaSource })),

    ingestCamera: "user",
    setIngestCamera: (ingestCamera: "user" | "environment") =>
      set(() => ({ ingestCamera })),

    ingestConnectionState: null,
    setIngestConnectionState: (
      ingestConnectionState: RTCPeerConnectionState | null,
    ) => set(() => ({ ingestConnectionState })),

    ingestAutoStart: true,
    setIngestAutoStart: (ingestAutoStart: boolean) =>
      set(() => ({ ingestAutoStart })),

    ingestStarted: null,
    setIngestStarted: (timestamp: number | null) =>
      set(() => ({ ingestStarted: timestamp })),

    stopIngest: () => {
      (set(() => ({
        ingestLive: false,
        ingestConnectionState: "new",
        ingestStarted: null,
      })),
        setTimeout(
          () =>
            set(() => ({
              ingestLive: false,
              ingestConnectionState: "new",
              ingestStarted: null,
            })),
          200,
        ));
    },

    fullscreen: false,
    setFullscreen: (isFullscreen: boolean) =>
      set(() => ({ fullscreen: isFullscreen })),

    status: PlayerStatus.START,
    setStatus: (status: PlayerStatus) => set(() => ({ status })),

    playTime: 0,
    setPlayTime: (playTime: number) => set(() => ({ playTime })),
    seekTo: (time: number) =>
      set((state) => {
        const ref = state.videoRef;
        if (ref && typeof ref === "object" && "current" in ref && ref.current) {
          ref.current.currentTime = time;
        }
        return { playTime: time };
      }),

    videoRef: undefined,
    setVideoRef: (
      videoRef:
        | React.MutableRefObject<HTMLVideoElement | null>
        | ((instance: HTMLVideoElement | null) => void)
        | null
        | undefined,
    ) => set(() => ({ videoRef })),

    pipMode: false,
    setPipMode: (pipMode: boolean) => set(() => ({ pipMode })),

    // Picture-in-Picture action function (set by player component)
    pipAction: undefined,
    setPipAction: (action: (() => void) | undefined) =>
      set(() => ({ pipAction: action })),

    togglePlayPause: () =>
      set((state) => {
        const ref = state.videoRef;
        if (ref && typeof ref === "object" && "current" in ref && ref.current) {
          if (ref.current.paused) {
            ref.current.play();
          } else {
            ref.current.pause();
          }
        }
        return {};
      }),
    setTogglePlayPause: (fn) => set(() => ({ togglePlayPause: fn })),

    // Player element width/height setters for global sync
    playerWidth: undefined,
    setPlayerWidth: (playerWidth: number) => set(() => ({ playerWidth })),
    playerHeight: undefined,
    setPlayerHeight: (playerHeight: number) => set(() => ({ playerHeight })),

    // * Whether mute was forced by the browser or not for autoplay
    // * Will get set to 'false' if the user has interacted with the volume
    muteWasForced: false,
    setMuteWasForced: (muteWasForced: boolean) =>
      set(() => ({ muteWasForced })),

    autoplayFailed: false,
    setAutoplayFailed: (autoplayFailed: boolean) =>
      set(() => ({ autoplayFailed })),

    embedded: false,
    setEmbedded: (embedded: boolean) => set(() => ({ embedded })),

    showControls: true,
    controlsTimeout: undefined,
    setShowControls: (showControls: boolean) =>
      set({ showControls, controlsTimeout: undefined }),

    telemetry: true,
    setTelemetry: (telemetry: boolean) => set(() => ({ telemetry })),

    ingestLive: false,
    setIngestLive: (ingestLive: boolean) => set(() => ({ ingestLive })),

    reportingURL: null,
    setReportingURL: (reportingURL: string | null) =>
      set(() => ({ reportingURL })),

    playerEvent: async (
      url: string,
      time: string,
      eventType: string,
      meta: { [key: string]: any },
    ) =>
      set((x) => {
        const data: PlayerEvent = {
          time: time,
          playerId: x.id,
          eventType: eventType,
          meta: {
            ...meta,
          },
        };
        try {
          // fetch url from sp provider
          const reportingURL = x.reportingURL ?? `${url}/api/player-event`;
          fetch(reportingURL, {
            method: "POST",
            body: JSON.stringify(data),
          });
        } catch (e) {
          console.error("error sending player telemetry", e);
        }
        return {};
      }),

    // Clear the controls timeout, if it exists.
    // Should be called on player unmount.
    clearControlsTimeout: () =>
      set((state) => {
        if (state.controlsTimeout) {
          clearTimeout(state.controlsTimeout);
        }
        return { controlsTimeout: undefined };
      }),

    setUserInteraction: () =>
      set((p) => {
        if (p.controlsTimeout) {
          clearTimeout(p.controlsTimeout);
        }
        if (p.selectedRendition === "audio") {
          return { showControls: true, controlsTimeout: undefined };
        }
        let controlsTimeout = setTimeout(() => p.setShowControls(false), 1000);
        return { showControls: true, controlsTimeout };
      }),

    showDebugInfo: false,
    setShowDebugInfo: (showDebugInfo: boolean) =>
      set(() => ({ showDebugInfo })),

    modMessage: null,
    setModMessage: (modMessage: ChatMessageViewHydrated | null) =>
      set(() => ({ modMessage })),

    reportModalOpen: false,
    setReportModalOpen: (reportModalOpen: boolean) =>
      set(() => ({ reportModalOpen })),

    reportSubject: null,
    setReportSubject: (
      subject: ComAtprotoModerationCreateReport.InputSchema["subject"] | null,
    ) => set(() => ({ reportSubject: subject })),
  }));

  // Load persisted protocol from storage asynchronously
  (async () => {
    try {
      const storedProtocol = await storage.getItem(PROTOCOL_STORAGE_KEY);
      if (
        storedProtocol &&
        Object.values(PlayerProtocol).includes(storedProtocol as PlayerProtocol)
      ) {
        store.setState({ protocol: storedProtocol as PlayerProtocol });
      }
    } catch (error) {
      console.error("Failed to load player protocol from storage:", error);
    }
  })();

  return store;
};

export function usePlayerContext() {
  const context = useContext(PlayerContext);
  if (!context) {
    throw new Error("usePlayerContext must be used within a PlayerProvider");
  }
  return context;
}

// Get a specific player store by ID
export function getPlayerStoreById(id: string): PlayerStore {
  const { players } = usePlayerContext();
  const playerStore = players[id];
  if (!playerStore) {
    throw new Error(`No player found with ID: ${id}`);
  }
  return playerStore;
}

// Will get the first player ID in the context
export function getFirstPlayerID(): string {
  const { players } = usePlayerContext();
  const playerIds = Object.keys(players);
  if (playerIds.length === 0) {
    throw new Error("No players found in context");
  }
  return playerIds[0];
}

export function getPlayerStoreFromContext(): PlayerStore {
  console.warn(
    "getPlayerStoreFromContext is deprecated. Use getPlayerStoreById instead.",
  );
  const { players } = usePlayerContext();
  const playerIds = Object.keys(players);
  if (playerIds.length === 0) {
    throw new Error("No players found in context");
  }
  return players[playerIds[0]];
}

// Use a specific player store by ID
// If no ID is provided, it will use the first player in the context
export function usePlayerStore<U>(
  selector: (state: PlayerState) => U,
  playerId?: string,
): U {
  if (!playerId) {
    playerId = Object.keys(usePlayerContext().players)[0];
  }
  const store = getPlayerStoreById(playerId);
  return useStore(store, selector);
}

/* Convenience selectors/hooks */
export const usePlayerProtocol = (
  playerId?: string,
): [PlayerProtocol, (protocol: PlayerProtocol) => void] =>
  usePlayerStore((x) => [x.protocol, x.setProtocol], playerId);

export const intoPlayerProtocol = (protocol: string): PlayerProtocol => {
  switch (protocol) {
    case "hls":
      return PlayerProtocol.HLS;
    case "progressive-mp4":
      return PlayerProtocol.PROGRESSIVE_MP4;
    case "progressive-webm":
      return PlayerProtocol.PROGRESSIVE_WEBM;
    default:
      return PlayerProtocol.WEBRTC;
  }
};

// returns true if the livestream has been offline for more than 10 seconds and we're not playing
export const useOffline = () => {
  const status = usePlayerStore((x) => x.status);
  const segment = useLivestreamStore((x) => x.segment);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const interval = setInterval(() => {
      setNow(Date.now());
    }, 500);
    return () => clearInterval(interval);
  }, []);
  if (status === PlayerStatus.PLAYING) {
    return false;
  }
  if (!segment?.startTime) {
    return false;
  }
  return now - Date.parse(segment.startTime) > 10000;
};

export const useIsMyStream = () => {
  const myHandle = useStreamplaceStore((state) => state.handle);
  const myDid = useStreamplaceStore((state) => state.oauthSession?.did);
  const channelId = usePlayerStore((state) => state.src);
  return () => {
    return myHandle === channelId || myDid === channelId;
  };
};
