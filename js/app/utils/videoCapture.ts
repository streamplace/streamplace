/**
 * Utility functions for capturing and compressing video frames
 */

/**
 * Captures a frame from a video element and returns it as a compressed PNG blob
 *
 * @param videoElement The video element to capture from
 * @param maxWidth Maximum width of the output image (maintains aspect ratio)
 * @param quality JPEG quality (0-1), lower means smaller file size
 * @returns A Promise that resolves to a Blob of the compressed image
 */
export const captureVideoFrame = async (
  videoElement: HTMLVideoElement,
  maxWidth = 1280,
  quality = 0.85,
): Promise<Blob> => {
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

export const findVideoElement = (): HTMLVideoElement | null => {
  return document.querySelector("video");
};
