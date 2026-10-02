import { PlayerMode } from "../../player-store/player-state";

export type PlayerProps = {
  name: string;
  playerId?: string;
  src: string;
  mode?: PlayerMode;
  muted: boolean;
  telemetry: boolean;
  fullscreen: boolean;
  setFullscreen: (isFullscreen: boolean) => void;
  ingest?: boolean;
  embedded?: boolean;
  reportingURL?: string;
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
  /** Seconds into a VOD to start playback at (from a `?t=` URL param). */
  startTime?: number;
};

export type VideoNativeProps = {
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
};
