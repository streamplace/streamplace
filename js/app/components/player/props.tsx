import { Rendition } from "lexicons/types/place/stream/defs";

export enum IngestMediaSource {
  USER = "user",
  DISPLAY = "display",
}

// common types shared by players and controls and stuff
export type PlayerProps = {
  name: string;
  src: string;
  muted: boolean;
  fullscreen: boolean;
  forceProtocol?: string;
  showControls: boolean;
  telemetry: boolean;
  setMuted: (isMuted: boolean) => void;
  setFullscreen: (isFullscreen: boolean) => void;
  userInteraction: () => void;
  playerEvent: (
    time: string,
    eventType: string,
    meta: { [key: string]: any },
  ) => void;
  playerId: string;
  status: PlayerStatus;
  setStatus: (status: PlayerStatus) => void;
  playTime: number;
  setPlayTime: (playTime: number) => void;
  ingest?: boolean;
  ingestMediaSource?: IngestMediaSource;
  ingestStreamKey?: string;
  ingestAutoStart?: boolean;
  avSyncTest?: boolean;
  offline: boolean;
  renditions: Rendition[];
  selectedRendition: string;
};

export type PlayerEvent = {
  id?: string;
  time: string;
  playerId: string;
  eventType: string;
  meta: { [key: string]: any };
};

export const PROTOCOL_HLS = "hls";
export const PROTOCOL_PROGRESSIVE_MP4 = "progressive-mp4";
export const PROTOCOL_PROGRESSIVE_WEBM = "progressive-webm";
export const PROTOCOL_WEBRTC = "webrtc";

export enum PlayerStatus {
  START = "start",
  PLAYING = "playing",
  STALLED = "stalled",
  SUSPEND = "suspend",
  WAITING = "waiting",
  PAUSE = "pause",
  MUTE = "mute",
}

export type PlayerStatusTracker = Partial<Record<PlayerStatus, number>>;
