import streamKey from "components/live-dashboard/stream-key";
import { selectStoredKey } from "features/bluesky/blueskySlice";
import { usePlayer } from "features/player/playerSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useCallback, useEffect, useState } from "react";
import { RTCView } from "react-native-webrtc";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { IngestMediaSource, PlayerProps } from "./props";
import { useWebRTCIngest } from "./use-webrtc";
import { mediaDevices } from "./webrtc-primitives";

export function WebcamIngestPlayer(props: PlayerProps) {
  const dispatch = useAppDispatch();
  const player = useAppSelector(usePlayer());
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
  const [localMediaStream, setLocalMediaStream] = useState<MediaStream | null>(
    null,
  );
  const [remoteMediaStream, setRemoteMediaStream] = useWebRTCIngest({
    endpoint: `${url}/api/ingest/webrtc`,
    streamKey: props.ingestStreamKey,
  });

  useEffect(() => {
    if (props.ingestMediaSource === IngestMediaSource.DISPLAY) {
      mediaDevices
        .getDisplayMedia({
          audio: true,
          video: true,
        })
        .then((stream) => {
          setLocalMediaStream(stream);
        })
        .catch((e) => {
          console.error("error getting display media", e);
        });
    } else {
      mediaDevices
        .getUserMedia({
          audio: true,
          video: {
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
  }, [props.ingestMediaSource]);

  useEffect(() => {
    if (!player.ingestStarting && !props.ingestAutoStart) {
      setRemoteMediaStream(null);
      return;
    }
    if (!localMediaStream) {
      return;
    }
    if (!streamKey) {
      return;
    }
    setRemoteMediaStream(localMediaStream);
  }, [
    localMediaStream,
    player.ingestStarting,
    streamKey,
    props.ingestAutoStart,
  ]);

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
      style={{ width: "100%", height: "100%" }}
    />
  );
}
