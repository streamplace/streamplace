import { useEffect, useState } from "react";
import type { place } from "streamplace";

// decodingInfo requires a bitrate, which the segment record doesn't carry: a
// rough AV1 estimate of 20 Mbps for 4K30, scaled by pixel rate.
const AV1_BITS_PER_PIXEL = 20_000_000 / (3840 * 2160 * 30);

/**
 * Whether this device decodes the stream's AV1 source track smoothly over
 * WebRTC, given the latest segment's first video track. A software decoder
 * can stutter on a 4K120 source; then the player should offer without AV1
 * (see offerWithoutAV1) so the node sends the stream's H.264 fallback.
 * True for non-AV1 tracks, before a segment arrives, and where the browser
 * can't tell (React Native, older browsers).
 */
export function useSmoothAV1(
  video: place.stream.segment.Video | null | undefined,
): boolean {
  const [smooth, setSmooth] = useState(true);
  const av1 = video?.codec === "av1";
  const width = video?.width ?? 0;
  const height = video?.height ?? 0;
  const num = video?.framerate?.num ?? 0;
  const den = video?.framerate?.den ?? 0;

  useEffect(() => {
    if (
      !av1 ||
      typeof navigator === "undefined" ||
      !navigator.mediaCapabilities?.decodingInfo
    ) {
      setSmooth(true);
      return;
    }
    let cancelled = false;
    const framerate = num && den ? num / den : 30;
    navigator.mediaCapabilities
      .decodingInfo({
        type: "webrtc",
        video: {
          contentType: "video/AV1",
          width,
          height,
          framerate,
          bitrate: Math.round(width * height * framerate * AV1_BITS_PER_PIXEL),
        },
      })
      .then(
        (info) => {
          if (cancelled) {
            return;
          }
          console.log(
            `AV1 ${width}x${height}@${framerate}: ${info.smooth ? "smooth, offering AV1" : "not smooth, offering without AV1"}`,
          );
          setSmooth(info.smooth);
        },
        (err) => console.warn("AV1 decodingInfo failed", err),
      );
    return () => {
      cancelled = true;
    };
  }, [av1, width, height, num, den]);

  return smooth;
}

/** Leaves AV1 out of a playback offer's video codecs, where the browser can. */
export function offerWithoutAV1(transceiver: RTCRtpTransceiver) {
  const codecs =
    typeof RTCRtpReceiver === "undefined"
      ? undefined
      : RTCRtpReceiver.getCapabilities("video")?.codecs;
  if (!codecs || !transceiver.setCodecPreferences) {
    return;
  }
  transceiver.setCodecPreferences(
    codecs.filter((c) => c.mimeType.toLowerCase() !== "video/av1"),
  );
}
