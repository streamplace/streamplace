import { useEffect, useRef, useState } from "react";
import * as sdpTransform from "sdp-transform";
import { place, StreamplaceAgent } from "streamplace";
import {
  PlayerStatus,
  useDID,
  useHandle,
  usePlayerStore,
  usePossiblyUnauthedPDSAgent,
  useStreamKey,
  useStreamplaceStore,
} from "../..";
import { useLivestreamStore } from "../../livestream-store";
import {
  AbrSample,
  AbrState,
  addAbrSample,
  createAbrState,
  decideRendition,
  syncAbrState,
} from "./webrtc-abr";
import { RTCPeerConnection, RTCSessionDescription } from "./webrtc-primitives";

/** How long to wait for the server to ack a rendition switch request. */
const SWITCH_ACK_TIMEOUT_MS = 5000;
/** After a rejected/timed-out switch, stop trying for this long. */
const SWITCH_FAILURE_SUPPRESS_MS = 5 * 60_000;

export default function useWebRTC(
  streamer: string,
): [MediaStream | null, boolean] {
  const [mediaStream, setMediaStream] = useState<MediaStream | null>(null);
  const [stuck, setStuck] = useState<boolean>(false);
  const setStatus = usePlayerStore((x) => x.setStatus);
  let agent = usePossiblyUnauthedPDSAgent();
  const myDID = useDID();
  const myHandle = useHandle();
  const playbackWorkerUrl = useStreamplaceStore(
    (state) => state.playbackWorkerUrl,
  );
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const setPlayingLiveRendition = usePlayerStore(
    (x) => x.setPlayingLiveRendition,
  );
  const liveRenditions = useLivestreamStore((x) => x.renditions);

  // In "auto" mode we always connect at source and the ABR controller then
  // switches renditions in-session over the "rendition" data channel. If the
  // channel never works (older server), we simply stay on source — the
  // highest quality — rather than reconnecting.
  const abrStateRef = useRef<AbrState | null>(null);
  const renditionChannelRef = useRef<RTCDataChannel | null>(null);
  const pendingSwitchRef = useRef<{
    timer: ReturnType<typeof setTimeout>;
  } | null>(null);
  const rendition = selectedRendition === "auto" ? "source" : selectedRendition;
  // The rendition the server is actually sending right now: the connected
  // rendition, updated on every applied in-session switch.
  const actualRenditionRef = useRef<string>(rendition);

  // Refs so the connection effect's interval always sees fresh values without
  // needing them in its dependency list (which would force a reconnect).
  const autoModeRef = useRef(selectedRendition === "auto");
  autoModeRef.current = selectedRendition === "auto";
  const liveRenditionsRef = useRef(liveRenditions);
  liveRenditionsRef.current = liveRenditions;

  // Leaving auto mode: throw away ABR bookkeeping and the menu readout. If
  // ABR had moved the server to a different rendition, request a switch back
  // to the now-manually-selected rendition over the data channel — the
  // connection effect won't re-run (auto and source both map to "source"),
  // so without this the server keeps streaming the ABR-selected rendition
  // while the UI shows the manual choice.
  useEffect(() => {
    if (selectedRendition === "auto") {
      return;
    }
    abrStateRef.current = null;
    setPlayingLiveRendition(null);
    const target =
      selectedRendition === "source" ? "source" : selectedRendition;
    if (actualRenditionRef.current !== target) {
      const channel = renditionChannelRef.current;
      if (channel?.readyState === "open") {
        try {
          channel.send(JSON.stringify({ rendition: target }));
        } catch (e) {
          console.warn(`[webrtc-abr] could not send manual switch: ${e}`);
        }
      }
    }
  }, [selectedRendition, setPlayingLiveRendition]);

  const isOwnStream = !!(
    myDID &&
    (streamer === myDID || streamer === myHandle)
  );

  const lastChange = useRef<number>(0);

  useEffect(() => {
    if (!agent) {
      return;
    }
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
      negotiateConnectionWithClientOffer(
        peerConnection,
        streamer,
        undefined,
        agent,
        isOwnStream,
        playbackWorkerUrl,
        rendition,
      );
    });

    // A new connection means we're back at the connected rendition; get the
    // ABR controller and the menu readout back in sync with reality.
    actualRenditionRef.current = rendition;
    const abr = abrStateRef.current;
    if (abr && abr.current !== rendition) {
      syncAbrState(abr, rendition, Date.now());
    }
    if (autoModeRef.current) {
      setPlayingLiveRendition(rendition);
    }
    if (pendingSwitchRef.current) {
      clearTimeout(pendingSwitchRef.current.timer);
      pendingSwitchRef.current = null;
    }

    // Channel used to ask the server to switch renditions in-session. On
    // older servers it may open but never answer; requests then time out
    // and we stay on the current rendition.
    const renditionChannel = peerConnection.createDataChannel("rendition");
    renditionChannelRef.current = renditionChannel;
    renditionChannel.addEventListener("message", (event) => {
      let msg: any;
      try {
        msg = JSON.parse(
          typeof event.data === "string"
            ? event.data
            : new TextDecoder().decode(event.data),
        );
      } catch {
        return;
      }
      if (msg.applied && typeof msg.rendition === "string") {
        actualRenditionRef.current = msg.rendition;
        setPlayingLiveRendition(msg.rendition);
        const abr = abrStateRef.current;
        if (abr && abr.current !== msg.rendition) {
          // e.g. a late ack after we already gave up on the request
          syncAbrState(abr, msg.rendition, Date.now());
        }
        console.log(`[webrtc-abr] now playing ${msg.rendition}`);
      } else if (msg.error) {
        console.warn(`[webrtc-abr] rendition switch rejected: ${msg.error}`);
        const abr = abrStateRef.current;
        if (abr) {
          syncAbrState(
            abr,
            actualRenditionRef.current,
            Date.now(),
            SWITCH_FAILURE_SUPPRESS_MS,
          );
        }
      } else {
        return;
      }
      if (pendingSwitchRef.current) {
        clearTimeout(pendingSwitchRef.current.timer);
        pendingSwitchRef.current = null;
      }
    });
    renditionChannel.addEventListener("close", () => {
      if (renditionChannelRef.current === renditionChannel) {
        renditionChannelRef.current = null;
      }
    });

    let lastFramesReceived = 0;
    let lastAudioFramesReceived = 0;

    const handle = setInterval(async () => {
      const stats = await peerConnection.getStats();
      let abrSample: AbrSample | null = null;
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
          abrSample = {
            at: Date.now(),
            bytesReceived: stat.bytesReceived ?? 0,
            packetsReceived: stat.packetsReceived ?? 0,
            packetsLost: stat.packetsLost ?? 0,
          };
        }
      });
      if (autoModeRef.current && abrSample) {
        const now = Date.now();
        const abr = (abrStateRef.current ??= createAbrState(
          actualRenditionRef.current,
          now,
        ));
        addAbrSample(abr, abrSample);
        const channel = renditionChannelRef.current;
        if (!pendingSwitchRef.current && channel?.readyState === "open") {
          const next = decideRendition(abr, liveRenditionsRef.current, now);
          if (next && next !== actualRenditionRef.current) {
            console.log(
              `[webrtc-abr] requesting switch ${actualRenditionRef.current} -> ${next}`,
            );
            pendingSwitchRef.current = {
              timer: setTimeout(() => {
                pendingSwitchRef.current = null;
                const stale = abrStateRef.current;
                if (stale) {
                  syncAbrState(
                    stale,
                    actualRenditionRef.current,
                    Date.now(),
                    SWITCH_FAILURE_SUPPRESS_MS,
                  );
                }
                console.warn("[webrtc-abr] rendition switch timed out");
              }, SWITCH_ACK_TIMEOUT_MS),
            };
            try {
              channel.send(JSON.stringify({ rendition: next }));
            } catch (e) {
              console.warn(`[webrtc-abr] could not send switch request: ${e}`);
              clearTimeout(pendingSwitchRef.current.timer);
              pendingSwitchRef.current = null;
              syncAbrState(
                abr,
                actualRenditionRef.current,
                Date.now(),
                SWITCH_FAILURE_SUPPRESS_MS,
              );
            }
          }
        }
      }
      if (Date.now() - lastChange.current > 2000) {
        setStuck(true);
      }
    }, 200);

    return () => {
      clearInterval(handle);
      renditionChannelRef.current = null;
      peerConnection.close();
    };
  }, [streamer, agent, isOwnStream, playbackWorkerUrl, rendition]);
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
  streamer: string,
  bearerToken?: string,
  agent?: StreamplaceAgent,
  isOwnStream?: boolean,
  playbackWorkerUrl?: string | null,
  rendition?: string,
) {
  /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/createOffer */
  const offer = await peerConnection.createOffer({
    offerToReceiveAudio: true,
    offerToReceiveVideo: true,
  });
  if (!offer.sdp) {
    throw Error("no SDP in offer");
  }

  const newSDP = forceStereoAudio(offer.sdp);

  offer.sdp = newSDP;
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
      let response = await postSDPOffer(
        streamer,
        ofr.sdp,
        bearerToken,
        agent,
        isOwnStream,
        playbackWorkerUrl,
        rendition,
      );
      let text = new TextDecoder().decode(response);
      if ((peerConnection.connectionState as string) === "closed") {
        return;
      }
      await peerConnection.setRemoteDescription(
        new RTCSessionDescription({ type: "answer", sdp: text }),
      );
      return "https://stream.place/example";
    } catch (e) {
      console.error(`posting sdp offer failed: ${e}`);
    }

    /** Limit reconnection attempts to at-most once every 5 seconds */
    await new Promise((r) => setTimeout(r, 5000));
  }
}

export async function negotiateIngestConnectionWithClientOffer(
  peerConnection: RTCPeerConnection,
  endpoint: string,
  bearerToken: string,
) {
  /** https://developer.mozilla.org/en-US/docs/Web/API/RTCPeerConnection/createOffer */
  const offer = await peerConnection.createOffer({
    offerToReceiveAudio: true,
    offerToReceiveVideo: true,
  });
  if (!offer.sdp) {
    throw Error("no SDP in offer");
  }

  const newSDP = forceStereoAudio(offer.sdp);

  offer.sdp = newSDP;
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
      let response = await postSDPIngestOffer(endpoint, ofr.sdp, bearerToken);

      if (response.status === 201) {
        if ((peerConnection.connectionState as string) === "closed") {
          return;
        }
        await peerConnection.setRemoteDescription(
          new RTCSessionDescription({
            type: "answer",
            sdp: await response.text(),
          }),
        );
        return "https://stream.place/example";
      } else {
        console.error(await response.text());
      }
    } catch (e) {
      console.error(`posting sdp offer failed: ${e}`);
    }

    /** Limit reconnection attempts to at-most once every 5 seconds */
    await new Promise((r) => setTimeout(r, 5000));
  }
}

async function getPlaybackServerAgent(
  agent: StreamplaceAgent,
  streamer: string,
  playbackWorkerUrl?: string | null,
): Promise<StreamplaceAgent> {
  if (!playbackWorkerUrl) {
    return agent;
  }

  try {
    const lookupAgent = new StreamplaceAgent(playbackWorkerUrl);
    const res = await lookupAgent.client.call(
      place.stream.playback.getPlaybackServer,
      { stream: streamer },
    );
    if (res.servers.length > 0) {
      const serverUrl = res.servers[0];
      console.log(`Using playback server: ${serverUrl}`);
      return new StreamplaceAgent(serverUrl);
    }
  } catch (e) {
    console.error("getPlaybackServer failed, using default agent:", e);
  }
  return agent;
}

async function postSDPOffer(
  streamer: string,
  data: string,
  bearerToken?: string,
  agent?: StreamplaceAgent,
  isOwnStream?: boolean,
  playbackWorkerUrl?: string | null,
  rendition?: string,
) {
  if (!agent) {
    throw new Error("No agent found");
  }
  // Own stream: use the authenticated PDS agent directly (needed for
  // unpublished stream preview). Otherwise, look up a playback server
  // and use an anonymous agent for it.
  const playbackAgent = isOwnStream
    ? agent
    : await getPlaybackServerAgent(agent, streamer, playbackWorkerUrl);
  return await playbackAgent.client.call(
    place.stream.playback.whep,
    data as any,
    {
      params: {
        rendition: rendition || "source",
        streamer: streamer,
      },
    },
  );
}

async function postSDPIngestOffer(
  endpoint: string,
  data: string,
  bearerToken: string,
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
      if (!storedKey?.streamKey?.privateKey) {
        throw new Error("no private key found");
      }
      negotiateIngestConnectionWithClientOffer(
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

export function forceStereoAudio(sdp: string): string {
  const parsedSDP = sdpTransform.parse(sdp);
  const audioMedia = parsedSDP.media.find((m) => m.type === "audio");
  if (!audioMedia) {
    throw Error("no audio media in SDP");
  }
  const opusCodec = audioMedia.rtp.find((c) => c.codec === "opus");
  if (!opusCodec) {
    throw Error("no opus codec in SDP");
  }
  const opusFMTP = audioMedia.fmtp.find((c) => c.payload === opusCodec.payload);
  if (!opusFMTP) {
    throw Error("no opus fmtp in SDP");
  }
  const opusParams = sdpTransform.parseParams(opusFMTP.config);
  opusParams.stereo = 1;
  const newParams = Object.entries(opusParams)
    .map(([k, v]) => `${k}=${v}`)
    .join(";");
  opusFMTP.config = newParams;
  return sdpTransform.write(parsedSDP);
}
