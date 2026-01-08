import { ComAtprotoModerationCreateReport } from "@atproto/api";
import { ChatMessageViewHydrated } from "streamplace";

export enum PlayerProtocol {
  WEBRTC = "webrtc",
  HLS = "hls",
  PROGRESSIVE_MP4 = "progressive-mp4",
  PROGRESSIVE_WEBM = "progressive-webm",
}

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

export enum IngestMediaSource {
  USER = "user",
  DISPLAY = "display",
}

export interface PlayerState {
  id: string;
  selectedRendition: string;
  setSelectedRendition: (rendition: string) => void;
  protocol: PlayerProtocol;
  setProtocol: (protocol: PlayerProtocol) => void;

  /** Source */
  src: string;

  /** Function to set the source URL */
  setSrc: (src: string) => void;

  /** Flag indicating if ingest (stream input) is currently starting */
  ingestStarting: boolean;

  /** Function to set the ingestStarting flag */
  setIngestStarting: (ingestStarting: boolean) => void;

  /** Flag indicating if ingest is live */
  ingestLive: boolean;
  setIngestLive: (ingestLive: boolean) => void;

  /** Current connection state of ingest RTP/RTC peer connection */
  ingestConnectionState: RTCPeerConnectionState | null;

  /** Function to update the ingest connection state */
  setIngestConnectionState: (state: RTCPeerConnectionState | null) => void;

  ingestMediaSource?: IngestMediaSource;
  setIngestMediaSource?: (source: IngestMediaSource) => void;

  ingestCamera: "user" | "environment";
  setIngestCamera: (camera: "user" | "environment") => void;

  ingestAutoStart?: boolean;
  setIngestAutoStart?: (autoStart: boolean) => void;

  /** Timestamp (number) when ingest started, or null if not started */
  ingestStarted: number | null;

  /** Function to set the ingestStarted timestamp */
  setIngestStarted: (timestamp: number | null) => void;

  /** Player fullscreen state */
  fullscreen: boolean;

  /** Function to set the fullscreen state */
  setFullscreen: (isFullscreen: boolean) => void;

  /** Current player status */
  status: PlayerStatus;

  /** Function to update the player status */
  setStatus: (status: PlayerStatus) => void;

  /** Current playback time in seconds */
  playTime: number;

  /** Function to set the current playback time */
  setPlayTime: (playTime: number) => void;

  /** Flag indicating if player is in offline state */
  /** Reference to the video element for direct manipulation (used for PiP) */
  videoRef:
    | React.MutableRefObject<HTMLVideoElement | null>
    | ((instance: HTMLVideoElement | null) => void)
    | null
    | undefined;

  /** Function to set the video reference */
  setVideoRef: (
    videoRef:
      | React.MutableRefObject<HTMLVideoElement | null>
      | ((instance: HTMLVideoElement | null) => void)
      | null
      | undefined,
  ) => void;

  pipAction: (() => void) | undefined;
  /** Function to set the Picture-in-Picture action */
  setPipAction: (action: (() => void) | undefined) => void;

  /** Player element width (CSS value or number) */
  playerWidth?: string | number;
  /** Function to set the player width */
  setPlayerWidth: (width: number) => void;

  /** Player element height (CSS value or number) */
  playerHeight?: string | number;
  /** Function to set the player height */
  setPlayerHeight: (height: number) => void;

  /** Flag indicating if player is in Picture-in-Picture mode */
  pipMode: boolean;

  /** Function to set the Picture-in-Picture mode */
  setPipMode: (pipMode: boolean) => void;

  /** Flag indicating if mute was forced by system (e.g., autoplay policy) */
  muteWasForced: boolean;

  /** Function to set the muteWasForced flag */
  setMuteWasForced: (muteWasForced: boolean) => void;

  /** Flag indicating if autoplay failed and needs user interaction */
  autoplayFailed: boolean;

  /** Function to set the autoplayFailed flag */
  setAutoplayFailed: (autoplayFailed: boolean) => void;

  /** Flag indicating if the player is embedded in another context */
  embedded: boolean;

  /** Function to set the embedded flag */
  setEmbedded: (embedded: boolean) => void;

  /** Flag indicating if player controls should be shown */
  showControls: boolean;
  controlsTimeout?: NodeJS.Timeout | undefined;

  /** Function to set the showControls flag */
  setShowControls: (showControls: boolean) => void;

  telemetry: boolean;
  setTelemetry: (telemetry: boolean) => void;

  playerEvent: (
    url: string,
    time: string,
    eventType: string,
    meta: { [key: string]: any },
  ) => void;

  clearControlsTimeout: () => void;

  setUserInteraction: () => void;

  showDebugInfo: boolean;
  setShowDebugInfo: (showDebugInfo: boolean) => void;

  /** Message to be moderated */
  modMessage: ChatMessageViewHydrated | null;

  /** Function to set the mod message */
  setModMessage: (message: ChatMessageViewHydrated | null) => void;

  /** URL to send player events to (if not default) */
  reportingURL: string | null;

  /** Function to set the reporting URL */
  setReportingURL: (reportingURL: string | null) => void;

  /** Is the report modal open? */
  reportModalOpen: boolean;

  /** Function to set the report modal open state */
  setReportModalOpen: (reportModalOpen: boolean) => void;

  /** Subject to report */
  reportSubject: ComAtprotoModerationCreateReport.InputSchema["subject"] | null;

  /** Function to set the report subject */
  setReportSubject: (
    subject: ComAtprotoModerationCreateReport.InputSchema["subject"] | null,
  ) => void;
}

export type PlayerEvent = {
  id?: string;
  time: string;
  playerId: string;
  eventType: string;
  meta: { [key: string]: any };
};
