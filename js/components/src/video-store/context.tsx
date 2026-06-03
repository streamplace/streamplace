import { createContext } from "react";
import { VideoStore } from "./video-store";

type VideoContextType = {
  store: VideoStore;
};

export const VideoContext = createContext<VideoContextType | null>(null);
