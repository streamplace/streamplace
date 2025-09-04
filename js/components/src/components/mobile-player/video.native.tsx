import { useEffect, useState } from "react";
import { View } from "react-native";
import { VideoNativeProps } from "./props";

let importPromise: Promise<typeof import("./video-async.native")> | null = null;

export default function VideoNative(props: VideoNativeProps) {
  if (!importPromise) {
    importPromise = import("./video-async.native");
  }

  const [videoNativeModule, setVideoNativeModule] = useState<
    typeof import("./video-async.native") | null
  >(null);

  useEffect(() => {
    importPromise?.then((module) => {
      setVideoNativeModule(module);
    });
  }, []);

  if (!videoNativeModule) {
    return <View></View>;
  }

  const VideoNative = videoNativeModule.default;

  return <VideoNative {...props} />;
}
