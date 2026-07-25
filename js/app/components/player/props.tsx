import { PlayerMode } from "@streamplace/components";

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
  videoRef:
    | React.MutableRefObject<HTMLVideoElement | null>
    | ((instance: HTMLVideoElement | null) => void)
    | null
    | undefined;
};
