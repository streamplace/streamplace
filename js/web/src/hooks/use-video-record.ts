// Fetches the hydrated video view used by the VOD page.
import { useEffect, useState } from "react";
import { place, StreamplaceAgent } from "streamplace";
import { useStore } from "../lib/store";

export type VideoView = place.stream.media.getVideo.VideoView;

export function useVideoRecord(user: string, tid: string) {
  const streamplaceUrl = useStore((state) => state.url);
  const anonPDSAgent = useStore((state) => state.anonPDSAgent);
  const [video, setVideo] = useState<VideoView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function fetchRecord() {
      setLoading(true);
      setError(null);

      try {
        const uri = `at://${user}/place.stream.video/${tid}`;
        const agent = anonPDSAgent ?? new StreamplaceAgent(streamplaceUrl);
        const result = await agent.client.call(place.stream.media.getVideo, {
          uri: uri as place.stream.media.getVideo.$Params["uri"],
        });

        if (!cancelled) setVideo(result as VideoView);
      } catch (caught) {
        if (!cancelled) {
          setVideo(null);
          setError(
            caught instanceof Error ? caught.message : "Failed to load video",
          );
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchRecord();
    return () => {
      cancelled = true;
    };
  }, [user, tid, anonPDSAgent, streamplaceUrl]);

  return { video, loading, error };
}
