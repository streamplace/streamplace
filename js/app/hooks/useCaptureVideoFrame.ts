import { usePlayerStore } from "@streamplace/components";
import { useCallback } from "react";
import { captureVideoFrame } from "utils/videoCapture";

/**
 * Hook to capture a frame from the video element
 *
 * This hook provides a function to capture a frame from the video element
 * that was stored in the VideoElementContext.
 *
 * @returns A function that captures a frame from the video element
 */
export function useCaptureVideoFrame() {
  const ref = usePlayerStore((state) => state.videoRef);

  // if ref is a function return null
  if (typeof ref === "function") {
    console.warn(
      "Video ref is a function (native player), cannot capture frame",
    );
    return null;
  }

  const videoElement = ref?.current;

  const captureFrame = useCallback(
    async (maxWidth = 1280, quality = 0.85): Promise<Blob | null> => {
      if (!videoElement) {
        console.warn("Video element not available or not on web platform");
        return null;
      }

      try {
        return await captureVideoFrame(videoElement, maxWidth, quality);
      } catch (error) {
        console.error("Error capturing frame:", error);
        return null;
      }
    },
    [videoElement],
  );

  return captureFrame;
}
