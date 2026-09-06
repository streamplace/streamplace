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
  const pdsAgent = useStore((state) => state.pdsAgent);
  const anonPDSAgent = useStore((state) => state.anonPDSAgent);
  const fetchBroadcasterDID = useStore((state) => state.fetchBroadcasterDID);
  const fetchBranding = useStore((state) => state.fetchBranding);

  useEffect(() => {
    if (!initialized) {
      initialize();
    }
  }, [initialized, initialize]);

  useEffect(() => {
    // wait until agent is ready
    if (!initialized || (!pdsAgent && !anonPDSAgent)) return;

    fetchBroadcasterDID().then(() => {
      fetchBranding({ force: false });
    });
  }, [initialized, pdsAgent, anonPDSAgent, fetchBroadcasterDID, fetchBranding]);

  return <>{children}</>;
}
