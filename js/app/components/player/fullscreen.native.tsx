import { useNavigation } from "@react-navigation/native";
import { VideoView } from "expo-video";
import { usePlayerProtocol } from "features/player/playerSlice";
import { useEffect, useRef, useState } from "react";
import { BackHandler, Dimensions, StatusBar, StyleSheet } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useAppSelector } from "store/hooks";
import { View } from "tamagui";
import Controls from "./controls";
import PlayerLoading from "./player-loading";
import { PlayerProps, PROTOCOL_WEBRTC } from "./props";
import VideoRetry from "./video-retry";
import Video from "./video.native";

// Standard 16:9 video aspect ratio
const VIDEO_ASPECT_RATIO = 16 / 9;

export default function Fullscreen(props: PlayerProps) {
  const ref = useRef<VideoView>(null);
  const insets = useSafeAreaInsets();
  const navigation = useNavigation();
  const [dimensions, setDimensions] = useState(Dimensions.get("window"));
  const protocol = useAppSelector(usePlayerProtocol());

  // Re-calculate dimensions on orientation change
  useEffect(() => {
    const updateDimensions = () => {
      setDimensions(Dimensions.get("window"));
    };

    const subscription = Dimensions.addEventListener(
      "change",
      updateDimensions,
    );

    return () => {
      subscription.remove();
    };
  }, []);

  // Hide status bar when in fullscreen mode
  useEffect(() => {
    if (props.fullscreen) {
      StatusBar.setHidden(true);
      console.log("setting sidebar hidden");

      // Hide the navigation header
      navigation.setOptions({
        headerShown: false,
      });

      // Handle hardware back button
      const backHandler = BackHandler.addEventListener(
        "hardwareBackPress",
        () => {
          props.setFullscreen(false);
          return true;
        },
      );

      return () => {
        backHandler.remove();
      };
    } else {
      StatusBar.setHidden(false);

      // Restore the navigation header
      navigation.setOptions({
        headerShown: true,
      });
    }

    return () => {
      StatusBar.setHidden(false);
      // Ensure header is restored if component unmounts
      navigation.setOptions({
        headerShown: true,
      });
    };
  }, [props.fullscreen, navigation]);

  const setFullscreen = (on) => {
    // For WebRTC, use custom fullscreen implementation
    if (protocol === PROTOCOL_WEBRTC) {
      props.setFullscreen(on);
      return;
    }

    // For HLS and other protocols, use native fullscreen
    if (ref.current) {
      if (on) {
        ref.current.enterFullscreen();
      } else {
        ref.current.exitFullscreen();
      }
    }
  };

  if (props.fullscreen && protocol === PROTOCOL_WEBRTC) {
    // Determine if we're in landscape mode
    const isLandscape = dimensions.width > dimensions.height;

    // Calculate video container dimensions based on screen size and orientation
    let videoWidth, videoHeight;

    if (isLandscape) {
      // In landscape, account for safe areas and use available height
      const availableHeight = dimensions.height - (insets.top + insets.bottom);
      const availableWidth = dimensions.width - (insets.left + insets.right);

      videoHeight = availableHeight;
      videoWidth = videoHeight * VIDEO_ASPECT_RATIO;

      // If calculated width exceeds available width, constrain and maintain aspect ratio
      if (videoWidth > availableWidth) {
        videoWidth = availableWidth;
        videoHeight = videoWidth / VIDEO_ASPECT_RATIO;
      }
    } else {
      // In portrait, account for safe areas
      const availableWidth = dimensions.width - (insets.left + insets.right);
      videoWidth = availableWidth;
      videoHeight = videoWidth / VIDEO_ASPECT_RATIO;
    }

    // Calculate position to center the video, accounting for safe areas
    const leftPosition = (dimensions.width - videoWidth) / 2;
    const topPosition = (dimensions.height - videoHeight) / 2;

    // When in custom fullscreen mode
    return (
      <View
        style={[
          styles.fullscreenContainer,
          {
            width: isLandscape ? dimensions.width + 40 : dimensions.width,
            height: dimensions.height,
          },
        ]}
      >
        <View
          style={[
            styles.videoContainer,
            {
              width: isLandscape ? videoWidth + 40 : videoWidth,
              height: videoHeight,
              left: leftPosition,
              top: topPosition,
            },
          ]}
        >
          <VideoRetry {...props}>
            <Video {...props} nativeVideoRef={ref} />
          </VideoRetry>
          <PlayerLoading {...props} />
          <Controls {...props} setFullscreen={setFullscreen} />
        </View>
      </View>
    );
  }

  // Normal non-fullscreen mode
  return (
    <>
      <PlayerLoading {...props}></PlayerLoading>
      <Controls {...props} setFullscreen={setFullscreen} />
      <VideoRetry {...props}>
        <Video {...props} nativeVideoRef={ref} />
      </VideoRetry>
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
    backgroundColor: "#000",
    zIndex: 9999,
    elevation: 9999,
    margin: 0,
    padding: 0,
    justifyContent: "center",
    alignItems: "center",
  },
  videoContainer: {
    position: "absolute",
    backgroundColor: "#111",
    overflow: "hidden",
  },
});
