import useAquareumNode from "hooks/useAquareumNode";
import {
  PlayerProps,
  PROTOCOL_HLS,
  PROTOCOL_PROGRESSIVE_MP4,
  PROTOCOL_PROGRESSIVE_WEBM,
  PROTOCOL_WEBRTC,
} from "./props";
import { useMemo } from "react";

const protocolSuffixes = {
  m3u8: PROTOCOL_HLS,
  mp4: PROTOCOL_PROGRESSIVE_MP4,
  webm: PROTOCOL_PROGRESSIVE_WEBM,
  webrtc: PROTOCOL_WEBRTC,
};

export function srcToUrl(props: PlayerProps): {
  url: string;
  protocol: string;
} {
  const { url } = useAquareumNode();
  return useMemo(() => {
    if (props.src.startsWith("http://") || props.src.startsWith("https://")) {
      const segments = props.src.split(/[./]/);
      const suffix = segments[segments.length - 1];
      if (protocolSuffixes[suffix]) {
        return {
          url: props.src,
          protocol: protocolSuffixes[suffix],
        };
      } else {
        throw new Error(`unknown playback protocol: ${suffix}`);
      }
    }
    let outUrl;
    if (props.protocol === PROTOCOL_HLS) {
      outUrl = `${url}/api/playback/${props.src}/hls/stream.m3u8`;
    } else if (props.protocol === PROTOCOL_PROGRESSIVE_MP4) {
      outUrl = `${url}/api/playback/${props.src}/stream.mp4`;
    } else if (props.protocol === PROTOCOL_PROGRESSIVE_WEBM) {
      outUrl = `${url}/api/playback/${props.src}/stream.webm`;
    } else if (props.protocol === PROTOCOL_WEBRTC) {
      outUrl = `${url}/api/playback/${props.src}/webrtc`;
    } else {
      throw new Error(`unknown playback protocol: ${props.protocol}`);
    }
    return {
      protocol: props.protocol,
      url: outUrl,
    };
  }, [props.src, props.protocol, url]);
}
