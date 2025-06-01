/**
 * Utility functions for capturing video frames
 */
import React from "react";
import { isWeb } from "tamagui";

/**
 * Captures a frame from a video ref or element and returns it as a compressed JPEG blob
 *
 * @param videoRefOrElement The video ref or element to capture from
 * @param maxWidth Maximum width of the output image (maintains aspect ratio)
 * @param quality JPEG quality (0-1), lower means smaller file size
 * @returns A Promise that resolves to a Blob of the compressed image
 */
export const captureVideoFrame = async (
  videoRefOrElement:
    | React.MutableRefObject<HTMLVideoElement | null>
    | HTMLVideoElement,
  maxWidth = 1280,
  quality = 0.85,
): Promise<Blob> => {
  let videoElement: HTMLVideoElement;

  if (
    videoRefOrElement &&
    typeof videoRefOrElement === "object" &&
    "current" in videoRefOrElement
  ) {
    if (!videoRefOrElement.current) {
      throw new Error("No video element available in ref");
    }
    videoElement = videoRefOrElement.current;
  } else {
    videoElement = videoRefOrElement as HTMLVideoElement;
  }

  if (!isWeb) {
    throw new Error("captureVideoFrame is only available on web platforms");
  }

  return new Promise((resolve, reject) => {
    try {
      const canvas = document.createElement("canvas");
      const ctx = canvas.getContext("2d");

      if (!ctx) {
        reject(new Error("Could not get canvas context"));
        return;
      }

      const videoWidth = videoElement.videoWidth;
      const videoHeight = videoElement.videoHeight;

      if (videoWidth === 0 || videoHeight === 0) {
        reject(new Error("Video has no dimensions, may not be playing yet"));
        return;
      }

      let width = videoWidth;
      let height = videoHeight;

      if (width > maxWidth) {
        const ratio = maxWidth / width;
        width = maxWidth;
        height = height * ratio;
      }

      canvas.width = width;
      canvas.height = height;

      ctx.drawImage(videoElement, 0, 0, width, height);

      canvas.toBlob(
        (blob) => {
          if (blob) {
            resolve(blob);
          } else {
            reject(new Error("Failed to create blob from canvas"));
          }
        },
        "image/jpeg",
        quality,
      );
    } catch (error) {
      reject(error);
    }
  });
};
