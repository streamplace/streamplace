import React, { createContext, useContext, ReactNode } from "react";

interface VideoElementContextType {
  videoElement: HTMLVideoElement | null;
}

const VideoElementContext = createContext<VideoElementContextType | undefined>(
  undefined,
);

export function VideoElementProvider({
  children,
  videoElement,
}: {
  children: ReactNode;
  videoElement: HTMLVideoElement | null;
}) {
  return (
    <VideoElementContext.Provider value={{ videoElement }}>
      {children}
    </VideoElementContext.Provider>
  );
}

export function useVideoElement() {
  const context = useContext(VideoElementContext);
  if (context === undefined) {
    throw new Error(
      "useVideoElement must be used within a VideoElementProvider",
    );
  }
  return context.videoElement;
}
