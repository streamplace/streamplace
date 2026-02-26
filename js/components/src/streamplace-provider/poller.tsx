import React, { useEffect } from "react";
import {
  useDID,
  useGetBskyProfile,
  useGetChatProfile,
  useStreamplaceStore,
} from "../streamplace-store";
import {
  usePDSAgent,
  usePossiblyUnauthedPDSAgent,
} from "../streamplace-store/xrpc";
import { useTimeSync } from "../time-sync";

export default function Poller({ children }: { children: React.ReactNode }) {
  const setLiveUsers = useStreamplaceStore((state) => state.setLiveUsers);
  const did = useDID();
  const pdsAgent = usePDSAgent();
  const getChatProfile = useGetChatProfile();
  const getBskyProfile = useGetBskyProfile();
  const liveUsersAgent = usePossiblyUnauthedPDSAgent();
  const liveUserRefresh = useStreamplaceStore(
    (state) => state.liveUsersRefresh,
  );

  useTimeSync();

  useEffect(() => {
    if (pdsAgent && did) {
      getChatProfile();
      getBskyProfile();
    }
  }, [pdsAgent, did]);

  useEffect(() => {
    if (!liveUsersAgent) return;
    const go = async () => {
      setLiveUsers({
        liveUsersLoading: true,
      });
      try {
        const res = await liveUsersAgent.place.stream.live.getLiveUsers();
        setLiveUsers({
          liveUsers: res.data.streams || [],
          liveUsersLoading: false,
          liveUsersError: null,
        });
      } catch (e) {
        setLiveUsers({
          liveUsersLoading: false,
          liveUsersError: e.message,
        });
      }
    };
    go();
    const handle = setInterval(go, 3000);
    return () => clearInterval(handle);
  }, [liveUsersAgent, liveUserRefresh]);

  return <>{children}</>;
}
