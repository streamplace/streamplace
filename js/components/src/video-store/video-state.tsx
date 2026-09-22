import { ChatMessageViewHydrated, VideoViewHydrated } from "streamplace";

// The chat of the livestream a video is the recording of, for a read-only
// replay beside it: every message oldest first, and the recording's start as
// a millisecond timestamp so a message belongs at (createdAt - startedAt)
// into the video.
export interface ChatReplay {
  messages: ChatMessageViewHydrated[];
  livestreams: string[];
  startedAt: number;
}

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

  // The livestream chat to replay beside the video, or null: not fetched
  // yet, or the video is not the recording of a livestream.
  replay: ChatReplay | null;
  setReplay: (replay: ChatReplay | null) => void;
}
