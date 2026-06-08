// Streamplace server URL provider. Calls `initialize()` on mount to
// hydrate the slice from storage, and publishes the URL via context
// for `useStreamplaceNode` consumers. The slice's `url` field is
// always populated (from `getStreamplaceUrl()` synchronously at
// module load), so this provider doesn't need to gate rendering on
// `initialized` — the slice reads the value lazily.
import { ReactNode, useEffect } from "react";
import {
  StreamplaceContext,
  StreamplaceNode,
} from "../../hooks/use-streamplace-node";
import { useStore } from "../store";
import { useStreamplaceInitialized } from "../store/hooks";

export default function StreamplaceProvider({
  children,
}: {
  children: ReactNode;
}) {
  const initialize = useStore((state) => state.initialize);
  const initialized = useStreamplaceInitialized();

  useEffect(() => {
    if (!initialized) {
      initialize();
    }
  }, [initialized, initialize]);

  const url = useStore((state) => state.url);
  const value: StreamplaceNode = { url };

  return (
    <StreamplaceContext.Provider value={value}>
      {children}
    </StreamplaceContext.Provider>
  );
}
