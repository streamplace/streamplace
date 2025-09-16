import { useEffect, useState } from "react";
import { Text, View } from "react-native";
import { VideoNativeProps } from "./props";

let importPromise: Promise<typeof import("./video-async.native")> | null = null;

export default function VideoNative(props: VideoNativeProps) {
  if (!importPromise) {
    importPromise = import("./video-async.native");
  }

  const [videoNativeModule, setVideoNativeModule] = useState<
    typeof import("./video-async.native") | null
  >(null);

  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    importPromise
      ?.then((module) => {
        setVideoNativeModule(module);
      })
      .catch((err) => {
        setError(err.message);
      });
  }, []);

  if (error) {
    console.error(error);
    return (
      <View style={{ flex: 1, justifyContent: "center", alignItems: "center" }}>
        <Text>{error}</Text>
      </View>
    );
  }

  if (!videoNativeModule) {
    return <View></View>;
  }

  const VideoNative = videoNativeModule.default;

  return <VideoNative {...props} />;
}
