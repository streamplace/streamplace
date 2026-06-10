// Streamplace server URL provider. Calls `initialize()` on mount to
// hydrate the slice from storage.
//
// Live-users polling now lives in hooks/use-live-users.ts (React Query)
// and is only active while the home route is mounted.
import { ReactNode, useEffect } from "react";
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

  return <>{children}</>;
}
