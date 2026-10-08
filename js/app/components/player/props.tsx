export type PlayerProps = {
  name: string;
  playerId?: string;
  src: string;
  mode?: "live" | "vod";
  /**
   * Temporal reference: initial playback position in seconds from the start
   * of the stream/video (YouTube-style #t=… fragment).
   */
  startTime?: number | null;
  muted: boolean;
  telemetry: boolean;
  fullscreen: boolean;
  setFullscreen: (isFullscreen: boolean) => void;
  ingest?: boolean;
  embedded?: boolean;
  videoRef:
    | React.MutableRefObject<HTMLVideoElement | null>
    | ((instance: HTMLVideoElement | null) => void)
    | null
    | undefined;
};
