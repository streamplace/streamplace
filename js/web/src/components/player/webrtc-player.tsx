// WebRTC backend for the <Player> component. Owns RTCPeerConnection setup,
// WHEP SDP negotiation, and reconnection. Renders nothing — the video element
// lives in <Player> so the chrome (controls, fullscreen, error display) is
// shared across backends. This is a sibling of hls-player.tsx with the same
// prop shape so the dispatch in PlayerBackend is a single conditional.
import { useEffect, useImperativeHandle, useRef, type RefObject } from "react";
import type { StreamplaceAgent } from "streamplace";
import { getStreamplaceUrl } from "../../lib/streamplace-url";
import type { PlayerBackendHandle, PlayerStats } from "./player";

export type WebRTCPlayerProps = {
  /** The video element managed by the parent <Player>. */
  videoRef: RefObject<HTMLVideoElement | null>;
  /**
   * The source URL. For WebRTC this is the same HLS playlist URL used by the
   * HLS backend — the streamer name is extracted from the `streamer` query
   * param. This keeps the <Player> interface transport-agnostic.
   */
  src: string;
  /** False tears down the peer connection and stops reconnection. */
  active: boolean;
  /** Called when the connection fails unrecoverably or encounters an error. */
  onError?: (message: string) => void;
  /** Called roughly once per second with a stats snapshot. */
  onStatsChange?: (stats: PlayerStats) => void;
};

const RECONNECT_DELAY_MS = 3000;
const STUCK_THRESHOLD_MS = 2000;
const ICE_GATHERING_TIMEOUT_MS = 1000;
const STATS_POLL_MS = 1000;

/**
 * Extracts the streamer handle/DID from a Streamplace playlist URL.
 * Falls back to the full URL origin + pathname if no `streamer` param exists.
 */
function extractStreamer(src: string): string {
  try {
    const url = new URL(src);
    const streamer = url.searchParams.get("streamer");
    if (streamer) return streamer;
  } catch {
    // not a valid URL, fall through
  }
  return src;
}

function extractServerUrl(src: string): string {
  try {
    const url = new URL(src);
    return url.origin;
  } catch {
    return getStreamplaceUrl();
  }
}

export function WebRTCPlayer({
  ref,
  videoRef,
  src,
  active,
  onError,
  onStatsChange,
}: WebRTCPlayerProps & {
  ref?: RefObject<PlayerBackendHandle | null>;
}) {
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const statsIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const agentRef = useRef<StreamplaceAgent | null>(null);

  // WebRTC doesn't have quality levels — expose a no-op so the chrome
  // doesn't break if the user clicks the quality menu.
  useImperativeHandle(
    ref ?? { current: null },
    () => ({
      setQuality: (_index: number) => {
        // no-op — WebRTC delivers a single rendition
      },
    }),
    [],
  );

  // Stable references that the reconnect loop can read without re-triggering
  // the main effect. We store them in refs so the inner async function
  // always sees the latest values without closing over stale ones.
  const activeRef = useRef(active);
  const onErrorRef = useRef(onError);
  const onStatsChangeRef = useRef(onStatsChange);
  activeRef.current = active;
  onErrorRef.current = onError;
  onStatsChangeRef.current = onStatsChange;

  useEffect(() => {
    if (!active) return;
    const video = videoRef.current;
    if (!video) return;

    const streamer = extractStreamer(src);
    const serverUrl = extractServerUrl(src);

    let cancelled = false;

    // Lazily create the agent — imported dynamically so the HLS-only path
    // never pays the import cost.
    async function getAgent(): Promise<StreamplaceAgent> {
      if (agentRef.current) return agentRef.current;
      const { StreamplaceAgent } = await import("streamplace");
      agentRef.current = new StreamplaceAgent(serverUrl);
      return agentRef.current;
    }

    async function connect() {
      if (cancelled) return;

      const peerConnection = new RTCPeerConnection({
        bundlePolicy: "max-bundle",
      });
      pcRef.current = peerConnection;

      // Receive both video and audio.
      peerConnection.addTransceiver("video", { direction: "recvonly" });
      peerConnection.addTransceiver("audio", { direction: "recvonly" });

      // When tracks arrive, attach the first stream to the video element.
      peerConnection.addEventListener("track", (event) => {
        if (cancelled) return;
        if (event.streams && event.streams[0]) {
          const v = videoRef.current;
          if (v) {
            v.srcObject = event.streams[0];
            v.play().catch(() => {});
          }
        }
      });

      // Detect disconnection and schedule a reconnect.
      peerConnection.addEventListener("connectionstatechange", () => {
        if (cancelled) return;
        const state = peerConnection.connectionState;
        if (
          state === "failed" ||
          state === "closed" ||
          state === "disconnected"
        ) {
          onErrorRef.current?.("Connection lost — reconnecting");
          scheduleReconnect();
        }
      });

      // Trigger negotiation when transceivers are added.
      peerConnection.addEventListener("negotiationneeded", () => {
        if (cancelled) return;
        negotiate(peerConnection, streamer);
      });

      // Start stats polling.
      startStatsPolling(peerConnection);

      // Clean up on unmount or when active goes false.
      const cleanup = () => {
        cancelled = true;
        stopStatsPolling();
        if (reconnectTimerRef.current) {
          clearTimeout(reconnectTimerRef.current);
          reconnectTimerRef.current = null;
        }
        peerConnection.close();
        pcRef.current = null;
      };

      // Store cleanup so the outer effect can call it.
      pendingCleanup.current = cleanup;
    }

    const pendingCleanup = { current: null as (() => void) | null };

    function scheduleReconnect() {
      if (cancelled) return;
      if (reconnectTimerRef.current) return;
      reconnectTimerRef.current = setTimeout(() => {
        reconnectTimerRef.current = null;
        if (!cancelled && activeRef.current) {
          // Tear down old connection, spin up a new one.
          pcRef.current?.close();
          pcRef.current = null;
          connect();
        }
      }, RECONNECT_DELAY_MS);
    }

    async function negotiate(pc: RTCPeerConnection, streamer: string) {
      try {
        const offer = await pc.createOffer({
          offerToReceiveAudio: true,
          offerToReceiveVideo: true,
        });
        if (!offer.sdp) {
          onErrorRef.current?.("Failed to create SDP offer");
          return;
        }

        await pc.setLocalDescription(offer);

        // Wait for ICE gathering to complete (or time out).
        const gatheredOffer = await waitForICE(pc);
        if (!gatheredOffer || !gatheredOffer.sdp) {
          onErrorRef.current?.("Failed to gather ICE candidates");
          return;
        }

        const agent = await getAgent();
        if (cancelled) return;

        // POST the offer via the WHEP endpoint.
        const response = await agent.place.stream.playback.whep(
          gatheredOffer.sdp,
          {
            qp: { rendition: "source", streamer },
          },
        );

        if (cancelled) return;

        const answerSdp =
          typeof response.data === "string"
            ? response.data
            : new TextDecoder().decode(response.data as BufferSource);

        await pc.setRemoteDescription(
          new RTCSessionDescription({ type: "answer", sdp: answerSdp }),
        );
      } catch (err) {
        if (cancelled) return;
        console.error("WebRTC negotiation failed:", err);
        onErrorRef.current?.("WebRTC negotiation failed");
        scheduleReconnect();
      }
    }

    function startStatsPolling(pc: RTCPeerConnection) {
      const lastFpsSample = { time: 0, frames: 0 };
      let lastFramesReceived = 0;
      let lastAudioReceived = 0;
      let lastBytesReceived = 0;
      let lastBitrateSampleTime = 0;
      let lastChangeTime = Date.now();
      let hasReceivedFrames = false;

      statsIntervalRef.current = setInterval(async () => {
        if (cancelled) return;
        try {
          const stats = await pc.getStats();
          let framesReceived = 0;
          let audioReceived = 0;
          let width = 0;
          let height = 0;
          let codec = "";
          let bytesReceived = 0;

          stats.forEach((report) => {
            if (report.type === "inbound-rtp") {
              const kind =
                (report as any).mediaType ?? (report as any).kind ?? "";
              if (kind === "video") {
                framesReceived = (report as any).framesReceived ?? 0;
                width = (report as any).frameWidth ?? 0;
                height = (report as any).frameHeight ?? 0;
                bytesReceived = (report as any).bytesReceived ?? 0;
                if ((report as any).codecId) {
                  const codecReport = (stats as Map<string, any>).get(
                    (report as any).codecId,
                  );
                  if (codecReport) {
                    codec = codecReport.mimeType ?? "";
                  }
                }
              }
              if (kind === "audio") {
                audioReceived =
                  (report as any).lastPacketReceivedTimestamp ?? 0;
              }
            }
          });

          // Stuck detection: if no new frames for STUCK_THRESHOLD_MS, the
          // stream is probably stalled. Only start checking after we've
          // received at least one frame — before that the connection is
          // still establishing.
          const now = Date.now();
          if (
            framesReceived !== lastFramesReceived ||
            audioReceived !== lastAudioReceived
          ) {
            lastFramesReceived = framesReceived;
            lastAudioReceived = audioReceived;
            lastChangeTime = now;
            hasReceivedFrames = true;
          }

          if (hasReceivedFrames && now - lastChangeTime > STUCK_THRESHOLD_MS) {
            onErrorRef.current?.("Stream stalled — reconnecting");
            scheduleReconnect();
            return;
          }

          // FPS from frame deltas.
          const perfNow = performance.now();
          let fps: number | undefined;
          if (lastFpsSample.time > 0) {
            const elapsed = perfNow - lastFpsSample.time;
            const deltaFrames = framesReceived - lastFpsSample.frames;
            if (elapsed > 0 && deltaFrames >= 0) {
              fps = (deltaFrames / elapsed) * 1000;
            }
          }
          lastFpsSample.time = perfNow;
          lastFpsSample.frames = framesReceived;

          // Bitrate from bytes-received delta.
          let bitrate: number | undefined;
          const elapsed = now - lastBitrateSampleTime;
          if (lastBitrateSampleTime > 0 && elapsed > 0) {
            const deltaBytes = bytesReceived - lastBytesReceived;
            if (deltaBytes >= 0) {
              bitrate = (deltaBytes * 8) / (elapsed / 1000);
            }
          }
          lastBitrateSampleTime = now;
          lastBytesReceived = bytesReceived;

          // Buffered: for WebRTC this is always ~0 (real-time), but we can
          // try to read from the video element.
          const video = videoRef.current;
          const buffered =
            video && video.buffered.length > 0
              ? video.buffered.end(video.buffered.length - 1) -
                video.currentTime
              : 0;

          onStatsChangeRef.current?.({
            width,
            height,
            viewportWidth:
              typeof window === "undefined" ? 0 : window.innerWidth,
            viewportHeight:
              typeof window === "undefined" ? 0 : window.innerHeight,
            buffered,
            droppedFrames: 0,
            totalFrames: framesReceived,
            fps,
            bitrate,
            level: -1,
            codecs: codec || undefined,
          });
        } catch {
          // getStats can throw if the connection is closing.
        }
      }, STATS_POLL_MS);
    }

    function stopStatsPolling() {
      if (statsIntervalRef.current) {
        clearInterval(statsIntervalRef.current);
        statsIntervalRef.current = null;
      }
    }

    connect();

    return () => {
      pendingCleanup.current?.();
    };
  }, [src, active, videoRef]);

  return null;
}

/**
 * Waits for ICE gathering to complete or times out after
 * ICE_GATHERING_TIMEOUT_MS.
 */
function waitForICE(
  pc: RTCPeerConnection,
): Promise<RTCSessionDescription | null> {
  return new Promise((resolve) => {
    const timeout = setTimeout(() => {
      if (pc.connectionState === "closed") return;
      resolve(pc.localDescription);
    }, ICE_GATHERING_TIMEOUT_MS);

    pc.addEventListener(
      "icegatheringstatechange",
      () => {
        if (pc.iceGatheringState === "complete") {
          clearTimeout(timeout);
          resolve(pc.localDescription);
        }
      },
      { once: true },
    );
  });
}
