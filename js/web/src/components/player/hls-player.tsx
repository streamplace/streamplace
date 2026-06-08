// HLS player using hls.js with native Safari fallback.
import Hls from "hls.js";
import { useCallback, useEffect, useRef, useState } from "react";

export type HLSPlayerProps = {
  /** Full URL to the master HLS playlist. */
  src: string;
  /** Optional poster image (e.g. /api/playback/:user/stream.jpg). */
  poster?: string;
  /** Optional poster that overrides the default when the stream is offline. */
  fallbackPoster?: string;
  /** True when a livestream is currently active. When false we just render the poster. */
  active: boolean;
  /**
   * Called when the player hits a fatal error. The parent may choose to
   * surface this as a `LivestreamProblem` in the store.
   */
  onError?: (message: string) => void;
  /** Called once playback has actually started. */
  onPlaying?: () => void;
};

export function HLSPlayer({
  src,
  poster,
  fallbackPoster,
  active,
  onError,
  onPlaying,
}: HLSPlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const hlsRef = useRef<Hls | null>(null);
  const [manifestReady, setManifestReady] = useState(false);
  const [hasPlayed, setHasPlayed] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const surfaceError = useCallback(
    (msg: string) => {
      setError(msg);
      onError?.(msg);
    },
    [onError],
  );

  useEffect(() => {
    if (!active) return;
    const video = videoRef.current;
    if (!video) return;

    setError(null);
    setHasPlayed(false);
    setManifestReady(false);

    if (Hls.isSupported()) {
      const hls = new Hls({
        maxAudioFramesDrift: 20,
      });
      hlsRef.current = hls;

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        setManifestReady(true);
        video.play().catch(() => {});
      });

      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return;
        const status = (data.response as Response | undefined)?.status;
        if (status === 404) {
          surfaceError("Stream not live");
          return;
        }
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            surfaceError("Network error — retrying");
            hls.startLoad();
            return;
          case Hls.ErrorTypes.MEDIA_ERROR:
            surfaceError("Media error — recovering");
            hls.recoverMediaError();
            return;
          default:
            surfaceError(`${data.type}: ${data.details}`);
            return;
        }
      });

      hls.loadSource(src);
      try {
        hls.attachMedia(video);
      } catch (e) {
        hls.stopLoad();
        return;
      }

      return () => {
        hls.destroy();
        hlsRef.current = null;
      };
    } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = src;
      const onCanPlay = () => {
        setManifestReady(true);
        video.play().catch(() => {});
        video.removeEventListener("canplay", onCanPlay);
      };
      video.addEventListener("canplay", onCanPlay);
      return () => {
        video.removeEventListener("canplay", onCanPlay);
        video.removeAttribute("src");
      };
    } else {
      surfaceError("Your browser doesn't support HLS playback.");
    }
  }, [src, active, surfaceError]);

  const onManualPlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    video.play().catch(() => {});
  }, []);

  const onVideoPlaying = useCallback(() => {
    setHasPlayed(true);
    setError(null);
    onPlaying?.();
  }, [onPlaying]);

  const onVideoError = useCallback(() => {
    surfaceError("Video element error");
  }, [surfaceError]);

  const showVideo = active;
  const showClickToPlay = showVideo && manifestReady && !hasPlayed && !error;

  return (
    <div className="relative w-full h-full bg-black">
      {showVideo ? (
        <video
          ref={videoRef}
          poster={poster}
          muted
          autoPlay
          playsInline
          controls
          className="absolute inset-0 w-full h-full object-contain"
          onPlaying={onVideoPlaying}
          onError={onVideoError}
        />
      ) : (
        <img
          src={fallbackPoster ?? poster}
          alt=""
          className="absolute inset-0 w-full h-full object-contain bg-black"
        />
      )}

      {showClickToPlay && (
        <button
          type="button"
          onClick={onManualPlay}
          className="absolute inset-0 flex items-center justify-center bg-black/40 hover:bg-black/30 transition-colors"
          aria-label="Click to play"
        >
          <div className="flex items-center gap-3 bg-white/10 backdrop-blur px-5 py-3 rounded-full border border-white/20">
            <div className="w-0 h-0 border-y-8 border-y-transparent border-l-[14px] border-l-white ml-1" />
            <span className="text-white font-medium">Click to play</span>
          </div>
        </button>
      )}

      {error && (
        <div className="absolute top-2 left-2 right-2 bg-red-500/90 text-white text-sm px-3 py-2 rounded pointer-events-auto">
          {error}
        </div>
      )}
    </div>
  );
}
