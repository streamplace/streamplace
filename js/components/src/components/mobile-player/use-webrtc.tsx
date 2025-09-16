import { useEffect, useRef, useState } from "react";
import { PlayerStatus, usePlayerStore, useStreamKey } from "../..";
import { RTCPeerConnection, RTCSessionDescription } from "./webrtc-primitives";

export default function useWebRTC(
  endpoint: string,
): [MediaStream | null, boolean] {
  const [mediaStream, setMediaStream] = useState<MediaStream | null>(null);
  const [stuck, setStuck] = useState<boolean>(false);
  const setStatus = usePlayerStore((x) => x.setStatus);

  const lastChange = useRef<number>(0);

  useEffect(() => {
    const peerConnection = new RTCPeerConnection({
      bundlePolicy: "max-bundle",
    });
    peerConnection.addTransceiver("video", {
      direction: "recvonly",
    });
    peerConnection.addTransceiver("audio", {
      direction: "recvonly",
    });
    peerConnection.addEventListener("track", (event) => {
      const track = event.track;
      if (!track) {
        return;
      }
      setMediaStream(event.streams[0]);
    });
    peerConnection.addEventListener("connectionstatechange", () => {
      console.log("connection state change", peerConnection.connectionState);
      if (
        peerConnection.connectionState === "closed" ||
        peerConnection.connectionState === "failed" ||
        peerConnection.connectionState === "disconnected"
      ) {
        console.log("setting stuck to true", peerConnection.connectionState);
        setStuck(true);
      }
      if (peerConnection.connectionState !== "connected") {
        return;
      }
    });
    peerConnection.addEventListener("negotiationneeded", () => {
      negotiateConnectionWithClientOffer(peerConnection, endpoint);
    });

    let lastFramesReceived = 0;
    let lastAudioFramesReceived = 0;

    const handle = setInterval(async () => {
      const stats = await peerConnection.getStats();
      stats.forEach((stat) => {
        const mediaType = stat.mediaType /* web */ ?? stat.kind; /* native */
        if (stat.type === "inbound-rtp" && mediaType === "audio") {
          const audioFramesReceived = stat.lastPacketReceivedTimestamp;
          if (lastAudioFramesReceived !== audioFramesReceived) {
            lastAudioFramesReceived = audioFramesReceived;
            lastChange.current = Date.now();
            setStatus(PlayerStatus.PLAYING);
            setStuck(false);
          }
        }
        if (stat.type === "inbound-rtp" && mediaType === "video") {
          const framesReceived = stat.framesReceived;
          if (lastFramesReceived !== framesReceived) {
            lastFramesReceived = framesReceived;
            lastChange.current = Date.now();
            setStatus(PlayerStatus.PLAYING);
            setStuck(false);
          }
        }
      });
      if (Date.now() - lastChange.current > 2000) {
        setStuck(true);
      }
    }, 200);

    return () => {
      clearInterval(handle);
      peerConnection.close();
    };
  }, [endpoint]);
  return [mediaStream, stuck];
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
  bearerToken?: string,
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
      let response = await postSDPOffer(`${endpoint}`, ofr.sdp, bearerToken);
      if (response.status === 201) {
        let answerSDP = await response.text();
        if ((peerConnection.connectionState as string) === "closed") {
          return;
        }
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

async function postSDPOffer(
  endpoint: string,
  data: string,
  bearerToken?: string,
) {
  return await fetch(endpoint, {
    method: "POST",
    mode: "cors",
    headers: {
      "content-type": "application/sdp",
      ...(bearerToken ? { Authorization: `Bearer ${bearerToken}` } : {}),
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

export function useWebRTCIngest({
  endpoint,
}: {
  endpoint: string;
}): [MediaStream | null, (mediaStream: MediaStream | null) => void] {
  const [mediaStream, setMediaStream] = useState<MediaStream | null>(null);
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  const setIngestConnectionState = usePlayerStore(
    (x) => x.setIngestConnectionState,
  );
  const storedKey = useStreamKey();
  useEffect(() => {
    if (storedKey?.error) {
      console.error("error creating stream key", storedKey.error);
    }
  }, [storedKey?.error]);
  const [peerConnection, setPeerConnection] =
    useState<RTCPeerConnection | null>(null);

  const videoTransceiver = useRef<RTCRtpTransceiver | null>(null);
  const audioTransceiver = useRef<RTCRtpTransceiver | null>(null);

  const [retryTime, setRetryTime] = useState<number>(0);
  const ingestLive = usePlayerStore((x) => x.ingestLive);

  // "Outer loop": when we need a new peer connection, this sets that up
  useEffect(() => {
    if (!storedKey) {
      return;
    }
    if (!ingestLive) {
      return;
    }
    const peerConnection = new RTCPeerConnection({
      bundlePolicy: "max-bundle",
    });

    videoTransceiver.current = peerConnection.addTransceiver("video", {
      direction: "sendonly",
    });
    audioTransceiver.current = peerConnection.addTransceiver("audio", {
      direction: "sendonly",
    });

    peerConnection.addEventListener("connectionstatechange", (ev) => {
      setIngestConnectionState(peerConnection.connectionState);
      console.log("connection state change", peerConnection.connectionState);
      if (peerConnection.connectionState === "failed") {
        setRetryTime(Date.now());
      }
    });
    peerConnection.addEventListener("negotiationneeded", (ev) => {
      negotiateConnectionWithClientOffer(
        peerConnection,
        endpoint,
        storedKey.streamKey?.privateKey,
      );
    });

    peerConnection.addEventListener("track", (ev) => {
      console.log(
        `got peerconnection track with ${ev.track.kind}`,
        ev.track.id,
      );
      // console.log(ev);
    });

    setPeerConnection(peerConnection);

    return () => {
      peerConnection.close();
    };
  }, [endpoint, storedKey.streamKey?.privateKey, retryTime, ingestLive]);

  // "Inner loop": when our tracks change, we update the transceivers
  useEffect(() => {
    if (!mediaStream) {
      return;
    }
    if (!peerConnection) {
      return;
    }
    if (!ingestLive) {
      return;
    }
    for (const track of mediaStream.getTracks()) {
      console.log(
        "adding track",
        track.kind,
        track.label,
        track.enabled,
        track.readyState,
      );
      if (track.kind === "video") {
        videoTransceiver.current?.sender?.replaceTrack(track);
      } else if (track.kind === "audio") {
        audioTransceiver.current?.sender?.replaceTrack(track);
      }
    }
  }, [peerConnection, mediaStream, ingestLive]);

  return [mediaStream, setMediaStream];
}
