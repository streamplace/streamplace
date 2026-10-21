import { ensureValidAtUri } from "@atproto/syntax";
import { useMemo } from "react";
import { usePlayerStore } from "../../player-store";
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
  // The streamer's own playback session, when their player holds one; it
  // opens their pre-live stream. Anyone else gets a session from the node.
  const playbackSession = usePlayerStore((x) => x.playbackSession);
  return useMemo(() => {
    if (props.src.startsWith("at://")) {
      ensureValidAtUri(props.src);
      const collection = props.src.split("/")[3];
      if (collection === "place.stream.video") {
        return {
          url: `${url}/xrpc/place.stream.playback.getVideoPlaylist?uri=${encodeURIComponent(props.src)}&ext=hls.m3u8`,
          protocol: PlayerProtocol.HLS,
        };
      } else {
        throw new Error(
          `this player doesn't know how to play ${collection} records (${props.src})`,
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
    const source = encodeURIComponent(props.src);
    let outUrl: string;
    if (protocol === PlayerProtocol.HLS) {
      if (
        props.selectedRendition &&
        props.selectedRendition !== "auto" &&
        props.selectedRendition !== "source"
      ) {
        outUrl = `${url}/xrpc/place.stream.playback.getLivePlaylist?streamer=${source}&rendition=${props.selectedRendition}`;
      } else {
        outUrl = `${url}/xrpc/place.stream.playback.getLivePlaylist?streamer=${source}`;
      }
      if (playbackSession) {
        outUrl += `&sid=${encodeURIComponent(playbackSession)}`;
      }
    } else if (protocol === PlayerProtocol.PROGRESSIVE_MP4) {
      outUrl = `${url}/api/playback/${source}/stream.mp4`;
    } else if (protocol === PlayerProtocol.PROGRESSIVE_WEBM) {
      outUrl = `${url}/api/playback/${source}/stream.webm`;
    } else if (protocol === PlayerProtocol.WEBRTC) {
      outUrl = `${url}/api/playback/${source}/webrtc?rendition=${props.selectedRendition || "source"}`;
    } else {
      throw new Error(`unknown playback protocol: ${protocol}`);
    }
    return {
      protocol: protocol,
      url: outUrl,
    };
  }, [props.src, props.selectedRendition, protocol, url, playbackSession]);
}
