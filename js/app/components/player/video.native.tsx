import { useVideoPlayer, VideoPlayerEvents, VideoView } from "expo-video";
import React, { useEffect, useState } from "react";
import {
  MediaStream,
  RTCPeerConnection,
  RTCSessionDescription,
  RTCView,
} from "react-native-webrtc";
import { View } from "tamagui";
import { PlayerProps, PlayerStatus, PROTOCOL_WEBRTC } from "./props";
import { srcToUrl } from "./shared";

// export function Player() {
//   return <View f={1}></View>;
// }

export default function NativeVideo(
  props: PlayerProps & { videoRef: React.RefObject<VideoView> },
) {
  if (props.protocol === PROTOCOL_WEBRTC) {
    return <NativeWHEP {...props} />;
  }
  const { url } = srcToUrl(props);
  useEffect(() => {
    return () => {
      props.setStatus(PlayerStatus.START);
    };
  }, []);
  const player = useVideoPlayer(url, (player) => {
    player.loop = true;
    player.muted = props.muted;
    player.play();
  });

  useEffect(() => {
    player.muted = props.muted;
  }, [props.muted, player]);

  useEffect(() => {
    const subs = (
      [
        "playToEnd",
        "playbackRateChange",
        "playingChange",
        "sourceChange",
        "statusChange",
        "volumeChange",
      ] as (keyof VideoPlayerEvents)[]
    ).map((evType) => {
      const now = new Date();
      return player.addListener(evType, (...args) => {
        props.playerEvent(now.toISOString(), evType, { args: args });
      });
    });

    subs.push(
      player.addListener("playingChange", (newIsPlaying) => {
        if (newIsPlaying) {
          props.setStatus(PlayerStatus.PLAYING);
        } else {
          props.setStatus(PlayerStatus.WAITING);
        }
      }),
    );

    return () => {
      for (const sub of subs) {
        sub.remove();
      }
    };
  }, [player]);

  return (
    <VideoView
      style={{ flex: 1, backgroundColor: "#111" }}
      ref={props.videoRef}
      player={player}
      allowsFullscreen
      allowsPictureInPicture
      nativeControls={props.fullscreen}
      onFullscreenEnter={() => {
        props.setFullscreen(true);
      }}
      onFullscreenExit={() => {
        props.setFullscreen(false);
      }}
    />
  );
}

export function NativeWHEP(props: PlayerProps) {
  const [client, setClient] = useState<NativeWHEPClient | null>(null);
  const [mediaStream, setMediaStream] = useState<MediaStream | null>(null);
  const { url } = srcToUrl(props);
  useEffect(() => {
    const client = new NativeWHEPClient(url, setMediaStream);
    setClient(client);
    return () => {
      client.close();
    };
  }, [url]);
  useEffect(() => {
    if (!mediaStream) {
      props.setStatus(PlayerStatus.WAITING);
      return;
    }
    props.setStatus(PlayerStatus.PLAYING);
  }, [mediaStream]);
  useEffect(() => {
    if (!mediaStream) {
      return;
    }
    mediaStream.getTracks().forEach((track) => {
      if (track.kind === "audio") {
        track._setVolume(props.muted ? 0 : 1);
      }
    });
  }, [mediaStream, props.muted]);
  if (!client || !mediaStream) {
    return <View></View>;
  }
  return (
    <RTCView
      mirror={false}
      objectFit={"contain"}
      streamURL={mediaStream.toURL()}
      style={{
        width: "100%",
        height: "100%",
        backgroundColor: "#111",
        flex: 1,
      }}
    />
  );
}

export class NativeWHEPClient {
  endpoint: string;
  peerConnection: RTCPeerConnection;
  setStream: (stream: MediaStream) => void;
  constructor(endpoint: string, setStream: (stream: MediaStream) => void) {
    console.log("WHEPClient constructor");
    this.endpoint = endpoint;
    this.setStream = setStream;
    /**
     * Create a new WebRTC connection, using public STUN servers with ICE,
     * allowing the client to disover its own IP address.
     * https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API/Protocols#ice
     */
    this.peerConnection = new RTCPeerConnection({
      // iceServers: [
      //   {
      //     urls: "stun:stun.cloudflare.com:3478",
      //   },
      // ],
      bundlePolicy: "max-bundle",
    });
    /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/addTransceiver */
    this.peerConnection.addTransceiver("video", {
      direction: "recvonly",
    });
    this.peerConnection.addTransceiver("audio", {
      direction: "recvonly",
    });
    /**
     * When new tracks are received in the connection, store local references,
     * so that they can be added to a MediaStream, and to the <video> element.
     *
     * https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/track_event
     */
    this.peerConnection.addEventListener("track", (event) => {
      const track = event.track;
      if (!track) {
        return;
      }
      this.setStream(event.streams[0]);
      // const currentTracks = this.stream.getTracks();
      // const streamAlreadyHasVideoTrack = currentTracks.some(
      //   (track) => track.kind === "video",
      // );
      // const streamAlreadyHasAudioTrack = currentTracks.some(
      //   (track) => track.kind === "audio",
      // );
    });
    this.peerConnection.addEventListener("connectionstatechange", (ev) => {
      console.log(
        "connection state change",
        this.peerConnection.connectionState,
      );
      if (this.peerConnection.connectionState !== "connected") {
        return;
      }
    });
    this.peerConnection.addEventListener("negotiationneeded", (ev) => {
      negotiateConnectionWithClientOffer(this.peerConnection, this.endpoint);
    });
  }

  close() {
    this.peerConnection.close();
  }
}

/**
 * Performs the actual SDP exchange.
 *
 * 1. Constructs the client's SDP offer
 * 2. Sends the SDP offer to the server,
 * 3. Awaits the server's offer.
 *
 * SDP describes what kind of media we can send and how the server and client communicate.
 *
 * https://developer.mozilla.org/en-US/docs/Glossary/SDP
 * https://www.ietf.org/archive/id/draft-ietf-wish-whip-01.html#name-protocol-operation
 */
export async function negotiateConnectionWithClientOffer(
  peerConnection: RTCPeerConnection,
  endpoint: string,
) {
  /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/createOffer */
  const offer = await peerConnection.createOffer({
    offerToReceiveAudio: true,
    offerToReceiveVideo: true,
  });
  /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/setLocalDescription */
  await peerConnection.setLocalDescription(offer);

  /** Wait for ICE gathering to complete */
  let ofr = await waitToCompleteICEGathering(peerConnection);
  if (!ofr) {
    throw Error("failed to gather ICE candidates for offer");
  }

  /**
   * As long as the connection is open, attempt to...
   */
  while (peerConnection.connectionState !== "closed") {
    try {
      /**
       * This response contains the server's SDP offer.
       * This specifies how the client should communicate,
       * and what kind of media client and server have negotiated to exchange.
       */
      console.log(`posting sdp offer: ${endpoint}`);
      let response = await postSDPOffer(endpoint, ofr.sdp);
      if (response.status === 201) {
        let answerSDP = await response.text();
        await peerConnection.setRemoteDescription(
          new RTCSessionDescription({ type: "answer", sdp: answerSDP }),
        );
        return response.headers.get("Location");
      } else if (response.status === 405) {
        console.log(
          "Remember to update the URL passed into the WHIP or WHEP client",
        );
      } else {
        const errorMessage = await response.text();
        console.error(errorMessage);
      }
    } catch (e) {
      console.error(`posting sdp offer failed: ${e}`);
    }

    /** Limit reconnection attempts to at-most once every 5 seconds */
    await new Promise((r) => setTimeout(r, 5000));
  }
}

async function postSDPOffer(endpoint: string, data: string) {
  return await fetch(endpoint, {
    method: "POST",
    mode: "cors",
    headers: {
      "content-type": "application/sdp",
    },
    body: data,
  });
}

/**
 * Receives an RTCPeerConnection and waits until
 * the connection is initialized or a timeout passes.
 *
 * https://www.ietf.org/archive/id/draft-ietf-wish-whip-01.html#section-4.1
 * https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/iceGatheringState
 * https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/icegatheringstatechange_event
 */
async function waitToCompleteICEGathering(peerConnection: RTCPeerConnection) {
  return new Promise<RTCSessionDescription | null>((resolve) => {
    /** Wait at most 1 second for ICE gathering. */
    setTimeout(function () {
      if (peerConnection.connectionState === "closed") {
        return;
      }
      resolve(peerConnection.localDescription);
    }, 1000);
    peerConnection.addEventListener("icegatheringstatechange", (ev) => {
      if (peerConnection.iceGatheringState === "complete") {
        resolve(peerConnection.localDescription);
      }
    });
  });
}
