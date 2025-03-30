import { VideoView } from "expo-video";
import React, { useRef, useEffect, useState } from "react";
import { StatusBar, Dimensions, StyleSheet, BackHandler } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useNavigation } from "@react-navigation/native";
import { View } from "tamagui";
import Controls from "./controls";
import PlayerLoading from "./player-loading";
import { PlayerProps } from "./props";
import Video from "./video.native";
import VideoRetry from "./video-retry";

// Standard 16:9 video aspect ratio
const VIDEO_ASPECT_RATIO = 16 / 9;

export default function Fullscreen(props: PlayerProps) {
  const ref = useRef<VideoView>(null);
  const insets = useSafeAreaInsets();
  const navigation = useNavigation();
  const [dimensions, setDimensions] = useState(Dimensions.get("window"));

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
    // Instead of using native fullscreen, just update our fullscreen state
    props.setFullscreen(on);
  };

  if (props.fullscreen) {
    // Determine if we're in landscape mode
    const isLandscape = dimensions.width > dimensions.height;

    // Calculate video container dimensions based on screen size and orientation
    let videoWidth, videoHeight;

    if (isLandscape) {
      // In landscape, the video takes the full height, width is calculated from aspect ratio
      videoHeight = dimensions.height + 0;
      videoWidth = videoHeight * VIDEO_ASPECT_RATIO;

      // If calculated width is greater than screen width, constrain to screen width
      if (videoWidth > dimensions.width) {
        videoWidth = dimensions.width;
        videoHeight = videoWidth / VIDEO_ASPECT_RATIO;
      }
    } else {
      // In portrait, the video takes the full width, height is calculated from aspect ratio
      videoWidth = dimensions.width;
      videoHeight = videoWidth / VIDEO_ASPECT_RATIO;
    }

    // Calculate position to center the video
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
            <Video {...props} videoRef={ref} />
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
        <Video {...props} videoRef={ref} />
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
