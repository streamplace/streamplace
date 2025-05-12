import { useCallback } from "react";
import { isWeb } from "tamagui";
import { useVideoElement } from "contexts/VideoElementContext";
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
  const videoElement = useVideoElement();

  const captureFrame = useCallback(
    async (maxWidth = 1280, quality = 0.85): Promise<Blob | null> => {
      if (!isWeb || !videoElement) {
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
