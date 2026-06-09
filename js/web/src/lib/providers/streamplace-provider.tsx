// Streamplace server URL provider. Calls `initialize()` on mount to
// hydrate the slice from storage, and publishes the URL via context
// for `useStreamplaceNode` consumers. Also polls the live-users list
// every 5s so the home feed is fresh without manual refresh.
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
  const fetchLiveUsers = useStore((state) => state.fetchLiveUsers);

  useEffect(() => {
    if (!initialized) {
      initialize();
    }
  }, [initialized, initialize]);

  // Poll live users every 5s. The slice's anonPDSAgent is lazily
  // constructed by fetchLiveUsers if BlueskyProvider hasn't yet run
  // loadOAuthClient, so the first fetch may create a throwaway agent.
  useEffect(() => {
    fetchLiveUsers();
    const handle = setInterval(fetchLiveUsers, 5000);
    return () => clearInterval(handle);
  }, [fetchLiveUsers]);

  const url = useStore((state) => state.url);
  const value: StreamplaceNode = { url };

  return (
    <StreamplaceContext.Provider value={value}>
      {children}
    </StreamplaceContext.Provider>
  );
}
