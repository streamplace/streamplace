import { createContext } from "react";
import { LivestreamStore } from "../livestream-store/livestream-store";

type LivestreamContextType = {
  store: LivestreamStore;
};

export const LivestreamContext = createContext<LivestreamContextType | null>(
  null,
);
