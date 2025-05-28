import { createContext } from "react";
import { StreamplaceStore } from "../streamplace-store/streamplace-store";

type StreamplaceContextType = {
  store: StreamplaceStore;
};

export const StreamplaceContext = createContext<StreamplaceContextType | null>(
  null,
);
