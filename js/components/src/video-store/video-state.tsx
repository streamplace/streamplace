import { VideoViewHydrated } from "streamplace";

export interface VideoState {
  // AT-URI of the place.stream.video record this store represents.
  aturi: string;
  setAturi: (aturi: string) => void;

  // Hydrated VOD metadata, or null until the first getVideo fetch resolves.
  video: VideoViewHydrated | null;
  setVideo: (video: VideoViewHydrated | null) => void;

  // True while a getVideo request is in flight.
  loading: boolean;
  setLoading: (loading: boolean) => void;

  // Last fetch error message, or null when the last fetch succeeded.
  error: string | null;
  setError: (error: string | null) => void;
}
