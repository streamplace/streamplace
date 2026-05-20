import { useMemo } from "react";
import { PlayerProtocol } from "../../player-store/player-state";
import { useStreamplaceStore } from "../../streamplace-store";

const protocolSuffixes = {
  m3u8: PlayerProtocol.HLS,
  mp4: PlayerProtocol.PROGRESSIVE_MP4,
  webm: PlayerProtocol.PROGRESSIVE_WEBM,
  webrtc: PlayerProtocol.WEBRTC,
};

export function srcToUrl(
  props: {
    src: string;
    selectedRendition?: string;
  },
  protocol: PlayerProtocol,
): {
  url: string;
  protocol: string;
} {
  const url = useStreamplaceStore((x) => x.url);
  return useMemo(() => {
    if (props.src.startsWith("http://") || props.src.startsWith("https://")) {
      const segments = props.src.split(/[./]/);
      const suffix = segments[segments.length - 1];
      return {
        url: props.src,
        protocol: protocolSuffixes[suffix] ?? PlayerProtocol.HLS,
      };
    }
    let outUrl: string;
    if (protocol === PlayerProtocol.HLS) {
      if (props.selectedRendition === "auto") {
        outUrl = `${url}/xrpc/place.stream.playback.getLivePlaylist?streamer=${props.src}`;
      } else {
        // todo: re-implement track selection here
        outUrl = `${url}/xrpc/place.stream.playback.getLivePlaylist?streamer=${props.src}`;
      }
    } else if (protocol === PlayerProtocol.PROGRESSIVE_MP4) {
      outUrl = `${url}/api/playback/${props.src}/stream.mp4`;
    } else if (protocol === PlayerProtocol.PROGRESSIVE_WEBM) {
      outUrl = `${url}/api/playback/${props.src}/stream.webm`;
    } else if (protocol === PlayerProtocol.WEBRTC) {
      outUrl = `${url}/api/playback/${props.src}/webrtc?rendition=${props.selectedRendition || "source"}`;
    } else {
      throw new Error(`unknown playback protocol: ${protocol}`);
    }
    return {
      protocol: protocol,
      url: outUrl,
    };
  }, [props.src, props.selectedRendition, protocol, url]);
}
