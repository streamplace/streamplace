import { VideoView } from "expo-video";
import { useEffect, useRef } from "react";
import {
  BackHandler,
  StyleSheet,
  useWindowDimensions,
  View,
} from "react-native";
import { SystemBars } from "react-native-edge-to-edge";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import {
  DanmuOverlay,
  PlayerProtocol,
  useDanmuEnabled,
  useDanmuLaneCount,
  useDanmuMaxMessages,
  useDanmuOpacity,
  useDanmuSpeed,
  useLivestreamStore,
  usePlayerStore,
  useSegmentDimensions,
  VideoRetry,
} from "../..";
import { colors, surfaces } from "../../lib/theme/tokens";
import { AudioOnlyOverlay } from "./ui/audio-only-overlay";
import Video from "./video.native";

export function Fullscreen(props: {
  src: string;
  children?: React.ReactNode;
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
}) {
  const ref = useRef<VideoView>(null);
  const insets = useSafeAreaInsets();
  const dimensions = useWindowDimensions();
  const videoDimensions = useSegmentDimensions();
  const videoAspectRatio =
    videoDimensions.width > 0 && videoDimensions.height > 0
      ? videoDimensions.width / videoDimensions.height
      : 16 / 9;

  // Get state from player store
  const protocol = usePlayerStore((x) => x.protocol);
  const fullscreen = usePlayerStore((x) => x.fullscreen);
  const setFullscreen = usePlayerStore((x) => x.setFullscreen);
  const handle = useLivestreamStore((x) => x.profile?.handle);

  const danmuEnabled = useDanmuEnabled();
  const danmuOpacity = useDanmuOpacity();
  const danmuSpeed = useDanmuSpeed();
  const danmuLaneCount = useDanmuLaneCount();
  const danmuMaxMessages = useDanmuMaxMessages();

  const setSrc = usePlayerStore((x) => x.setSrc);

  useEffect(() => {
    setSrc(props.src);
  }, [props.src]);

  // Hide status bar when in fullscreen mode
  useEffect(() => {
    if (fullscreen) {
      SystemBars.setHidden(true);
      console.log("setting sidebar hidden");

      // Handle hardware back button
      const backHandler = BackHandler.addEventListener(
        "hardwareBackPress",
        () => {
          setFullscreen(false);
          return true;
        },
      );

      return () => {
        backHandler.remove();
      };
    } else {
      SystemBars.setHidden(false);
    }

    return () => {
      SystemBars.setHidden(false);
    };
  }, [fullscreen, setFullscreen]);

  // Handle fullscreen state changes for native video players
  useEffect(() => {
    // For WebRTC, we handle fullscreen manually via the custom implementation
    if (protocol === PlayerProtocol.WEBRTC) {
      return;
    }

    // For HLS and other protocols, sync with native fullscreen
    if (ref.current) {
      if (fullscreen) {
        ref.current.enterFullscreen();
      } else {
        ref.current.exitFullscreen();
      }
    }
  }, [fullscreen, protocol]);

  if (fullscreen && protocol === PlayerProtocol.WEBRTC) {
    // Fit the stream inside the safe area without cropping in either orientation.
    const availableWidth = dimensions.width - insets.left - insets.right;
    const availableHeight = dimensions.height - insets.top - insets.bottom;
    const videoWidth = Math.min(
      availableWidth,
      availableHeight * videoAspectRatio,
    );
    const videoHeight = videoWidth / videoAspectRatio;
    const leftPosition = insets.left + (availableWidth - videoWidth) / 2;
    const topPosition = insets.top + (availableHeight - videoHeight) / 2;

    // When in custom fullscreen mode
    return (
      <View
        style={[
          styles.fullscreenContainer,
          {
            width: dimensions.width,
            height: dimensions.height,
          },
        ]}
      >
        <View
          style={[
            styles.videoContainer,
            {
              width: videoWidth,
              height: videoHeight,
              left: leftPosition,
              top: topPosition,
            },
          ]}
        >
          <Video
            objectFit={props.objectFit}
            pictureInPictureEnabled={props.pictureInPictureEnabled}
          />
          <AudioOnlyOverlay />
          <DanmuOverlay
            enabled={danmuEnabled}
            opacity={danmuOpacity}
            speed={danmuSpeed}
            laneCount={danmuLaneCount}
            maxMessages={danmuMaxMessages}
          />
          {props.children}
        </View>
      </View>
    );
  }

  // Normal non-fullscreen mode
  return (
    <>
      <VideoRetry>
        <Video
          objectFit={props.objectFit}
          pictureInPictureEnabled={props.pictureInPictureEnabled}
        />
      </VideoRetry>
      <AudioOnlyOverlay />
      <DanmuOverlay
        enabled={danmuEnabled}
        opacity={danmuOpacity}
        speed={danmuSpeed}
        laneCount={danmuLaneCount}
        maxMessages={danmuMaxMessages}
      />
      {props.children}
    </>
  );
}

const styles = StyleSheet.create({
  fullscreenContainer: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: colors.black,
    zIndex: 9999,
    elevation: 9999,
    margin: 0,
    padding: 0,
    justifyContent: "center",
    alignItems: "center",
  },
  videoContainer: {
    position: "absolute",
    backgroundColor: surfaces.dark[1],
    overflow: "hidden",
  },
});
