// common types shared by players and controls and stuff
export type PlayerProps = {
  name: string;
  src: string;
  muted: boolean;
  fullscreen: boolean;
  protocol: string;
  showControls: boolean;
  telemetry: boolean;
  setMuted: (isMuted: boolean) => void;
  setFullscreen: (isFullscreen: boolean) => void;
  setProtocol: (protocol: string) => void;
  userInteraction: () => void;
  playerEvent: (
    time: string,
    eventType: string,
    meta: { [key: string]: any },
  ) => void;
  playerId: string;
  status: PlayerStatus;
  setStatus: (status: PlayerStatus) => void;
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

export enum PlayerStatus {
  START = "start",
  PLAYING = "playing",
  STALLED = "stalled",
  WAITING = "waiting",
}

export type PlayerStatusTracker = Partial<Record<PlayerStatus, number>>;
