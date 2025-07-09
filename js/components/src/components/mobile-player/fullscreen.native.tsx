import { useNavigation } from "@react-navigation/native";
import { VideoView } from "expo-video";
import { useEffect, useRef, useState } from "react";
import { BackHandler, Dimensions, StyleSheet, View } from "react-native";
import { SystemBars } from "react-native-edge-to-edge";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { PlayerProtocol, useLivestreamStore, usePlayerStore } from "../..";
import Video from "./video.native";

// Standard 16:9 video aspect ratio
const VIDEO_ASPECT_RATIO = 16 / 9;

export function Fullscreen(props: { src: string; children?: React.ReactNode }) {
  const ref = useRef<VideoView>(null);
  const insets = useSafeAreaInsets();
  const navigation = useNavigation();
  const [dimensions, setDimensions] = useState(Dimensions.get("window"));

  // Get state from player store
  const protocol = usePlayerStore((x) => x.protocol);
  const fullscreen = usePlayerStore((x) => x.fullscreen);
  const setFullscreen = usePlayerStore((x) => x.setFullscreen);
  const handle = useLivestreamStore((x) => x.profile?.handle);

  const setSrc = usePlayerStore((x) => x.setSrc);

  useEffect(() => {
    setSrc(props.src);
  }, [props.src]);

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
    if (fullscreen) {
      SystemBars.setHidden(true);
      console.log("setting sidebar hidden");

      // Hide the navigation header
      navigation.setOptions({
        headerShown: false,
      });

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

      // Restore the navigation header
      navigation.setOptions({
        headerShown: true,
      });
    }

    return () => {
      SystemBars.setHidden(false);
      // Ensure header is restored if component unmounts
      navigation.setOptions({
        headerShown: true,
      });
    };
  }, [fullscreen, navigation, setFullscreen]);

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
    // Determine if we're in landscape mode
    const isLandscape = dimensions.width > dimensions.height;

    // Calculate video container dimensions based on screen size and orientation
    let videoWidth: number;
    let videoHeight: number;

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
          <Video />
          {props.children}
        </View>
      </View>
    );
  }

  // Normal non-fullscreen mode
  return (
    <>
      <Video />
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
