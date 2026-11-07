import { useEffect, useState } from "react";
import type { place } from "streamplace";

// decodingInfo requires a bitrate, which the segment record doesn't carry: a
// rough AV1 estimate of 20 Mbps for 4K30, scaled by pixel rate.
const AV1_BITS_PER_PIXEL = 20_000_000 / (3840 * 2160 * 30);

/**
 * Whether this device should take the stream's AV1 source track over
 * WebRTC, given the latest segment's video tracks (the source first). A
 * software decoder can stutter on a 4K120 source; then the player should
 * offer without AV1 (see offerWithoutAV1) so the node sends the stream's
 * H.264 fallback. Where the device can't tell (React Native, browsers
 * without MediaCapabilities for WebRTC), it takes the fallback too: phones
 * decode H.264 in hardware. A stream without a fallback keeps AV1 either
 * way. True for non-AV1 sources and before a segment arrives.
 */
export function useSmoothAV1(
  videos: place.stream.segment.Video[] | null | undefined,
): boolean {
  const [smooth, setSmooth] = useState(true);
  const source = videos?.[0];
  const av1 = source?.codec === "av1";
  const hasH264 = !!videos?.some((v) => v.codec === "h264");
  const width = source?.width ?? 0;
  const height = source?.height ?? 0;
  const num = source?.framerate?.num ?? 0;
  const den = source?.framerate?.den ?? 0;

  useEffect(() => {
    if (!av1) {
      setSmooth(true);
      return;
    }
    const cantTell = (why: string) => {
      console.log(
        `AV1 ${width}x${height}: ${why}, ${hasH264 ? "offering without AV1 for the H.264 fallback" : "offering AV1, the stream has no fallback"}`,
      );
      setSmooth(!hasH264);
    };
    if (
      typeof navigator === "undefined" ||
      !navigator.mediaCapabilities?.decodingInfo
    ) {
      cantTell("no MediaCapabilities");
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
          // Without a fallback, a struggling decoder still beats no video.
          const take = info.smooth || !hasH264;
          console.log(
            `AV1 ${width}x${height}@${framerate}: ${info.smooth ? "smooth" : "not smooth"}, ${take ? "offering AV1" : "offering without AV1 for the H.264 fallback"}`,
          );
          setSmooth(take);
        },
        (err) => {
          if (!cancelled) {
            cantTell(`decodingInfo failed (${err})`);
          }
        },
      );
    return () => {
      cancelled = true;
    };
  }, [av1, hasH264, width, height, num, den]);

  return smooth;
}

/**
 * Leaves AV1 out of a playback offer's video codecs, where the platform can.
 * receiver is the platform's RTCRtpReceiver (react-native-webrtc's on
 * native, where it isn't a global).
 */
export function offerWithoutAV1(
  transceiver: RTCRtpTransceiver,
  receiver: Pick<typeof RTCRtpReceiver, "getCapabilities"> | undefined,
) {
  const codecs = receiver?.getCapabilities("video")?.codecs;
  if (!codecs || !transceiver.setCodecPreferences) {
    return;
  }
  transceiver.setCodecPreferences(
    codecs.filter((c) => c.mimeType.toLowerCase() !== "video/av1"),
  );
}
