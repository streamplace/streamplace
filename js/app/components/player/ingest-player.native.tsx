import {
  IngestMediaSource,
  PlayerStatus,
  usePlayerStore,
} from "@streamplace/components";
import streamKey from "components/live-dashboard/stream-key";
import { selectStoredKey } from "features/bluesky/blueskySlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useCallback, useEffect, useState } from "react";
import { RTCView } from "react-native-webrtc";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { useWebRTCIngest } from "./use-webrtc";
import { mediaDevices, WebRTCMediaStream } from "./webrtc-primitives.native";

export function NativeIngestPlayer() {
  const ingestStarting = usePlayerStore((x) => x.ingestStarting);
  const ingestMediaSource = usePlayerStore((x) => x.ingestMediaSource);
  const ingestAutoStart = usePlayerStore((x) => x.ingestAutoStart);
  const setStatus = usePlayerStore((x) => x.setStatus);
  useEffect(() => {
    setStatus(PlayerStatus.PLAYING);
  }, [setStatus]);
  const dispatch = useAppDispatch();
  const storedKey = useAppSelector(selectStoredKey);
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const handleRef = useCallback((node: HTMLVideoElement | null) => {
    if (node) {
      setVideoElement(node);
    }
  }, []);

  const { url } = useStreamplaceNode();
  const [localMediaStream, setLocalMediaStream] =
    useState<WebRTCMediaStream | null>(null);
  const [remoteMediaStream, setRemoteMediaStream] = useWebRTCIngest({
    endpoint: `${url}/api/ingest/webrtc`,
  });

  useEffect(() => {
    if (ingestMediaSource === IngestMediaSource.DISPLAY) {
      mediaDevices
        .getDisplayMedia()
        .then((stream) => {
          console.log("display media", stream);
          setLocalMediaStream(stream);
        })
        .catch((e) => {
          console.log("error getting display media", e);
          console.error("error getting display media", e);
        });
    } else {
      mediaDevices
        .getUserMedia({
          audio: {
            // deviceId: "audio-1",
            echoCancellation: false,
            autoGainControl: false,
            noiseSuppression: false,
            // latency: false,
            // channelCount: false,
          },
          video: {
            // deviceId: "1",
            width: { min: 200, ideal: 1920, max: 3840 },
            height: { min: 200, ideal: 1080, max: 2160 },
          },
        })
        .then((stream) => {
          setLocalMediaStream(stream);
        })
        .catch((e) => {
          console.error("error getting user media", e);
        });
    }
  }, [ingestMediaSource]);

  useEffect(() => {
    if (!ingestStarting && !ingestAutoStart) {
      setRemoteMediaStream(null);
      return;
    }
    if (!localMediaStream) {
      return;
    }
    if (!streamKey) {
      return;
    }
    console.log("setting remote media stream", localMediaStream);
    setRemoteMediaStream(localMediaStream);
  }, [localMediaStream, ingestStarting, streamKey, ingestAutoStart]);

  useEffect(() => {
    if (!videoElement) {
      return;
    }
    if (!localMediaStream) {
      return;
    }
    videoElement.srcObject = localMediaStream;
  }, [videoElement, localMediaStream]);

  if (!localMediaStream) {
    return null;
  }

  return (
    <RTCView
      mirror={true}
      objectFit={"cover"}
      streamURL={localMediaStream.toURL()}
      zOrder={0}
      style={{ width: "100%", height: "100%", backgroundColor: "green" }}
    />
  );
}
