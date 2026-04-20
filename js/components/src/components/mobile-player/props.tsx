export type PlayerProps = {
  name: string;
  playerId?: string;
  src: string;
  mode?: "live" | "vod";
  muted: boolean;
  telemetry: boolean;
  fullscreen: boolean;
  setFullscreen: (isFullscreen: boolean) => void;
  ingest?: boolean;
  embedded?: boolean;
  reportingURL?: string;
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
};

export type VideoNativeProps = {
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
};
