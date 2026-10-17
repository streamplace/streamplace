// React Context for the livestream store. Lives in @streamplace/components
// because it's a React-specific binding; the store itself is in
// @streamplace/core.
import type { LivestreamStore } from "@streamplace/core";
import { createContext } from "react";

type LivestreamContextType = {
  store: LivestreamStore;
};

export const LivestreamContext = createContext<LivestreamContextType | null>(
  null,
);
