import { AtUri } from "@atproto/syntax";
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
    if (props.src.startsWith("at://")) {
      const aturi = new AtUri(props.src);
      if (aturi.collection === "place.stream.video") {
        return {
          url: `${url}/xrpc/place.stream.playback.getVideoPlaylist?uri=${props.src}&ext=hls.m3u8`,
          protocol: PlayerProtocol.HLS,
        };
      } else {
        throw new Error(
          `this player doesn't know how to play ${aturi.collection} records (${props.src})`,
        );
      }
    }
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
      const llhlsUrl = `${url}/api/playback/${encodeURIComponent(props.src)}/llhls/master.m3u8`;
      if (
        props.selectedRendition &&
        props.selectedRendition !== "auto" &&
        props.selectedRendition !== "source"
      ) {
        outUrl = `${url}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(props.src)}&rendition=${encodeURIComponent(props.selectedRendition)}`;
      } else {
        outUrl = llhlsUrl;
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
