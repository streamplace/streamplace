import React, { useEffect } from "react";
import { StreamplaceAgent } from "streamplace";
import {
  useDID,
  useGetBskyProfile,
  useGetChatProfile,
  useStreamplaceStore,
} from "../streamplace-store";
import { usePDSAgent } from "../streamplace-store/xrpc";

export default function Poller({ children }: { children: React.ReactNode }) {
  const url = useStreamplaceStore((state) => state.url);
  const setLiveUsers = useStreamplaceStore((state) => state.setLiveUsers);
  const did = useDID();
  const pdsAgent = usePDSAgent();
  const getChatProfile = useGetChatProfile();
  const getBskyProfile = useGetBskyProfile();

  useEffect(() => {
    if (pdsAgent && did) {
      getChatProfile();
      getBskyProfile();
    }
  }, [pdsAgent, did]);

  useEffect(() => {
    const agent = new StreamplaceAgent(url);
    const go = async () => {
      const res = await agent.place.stream.live.getLiveUsers();
      setLiveUsers(res.data.streams || []);
    };
    go();
    const handle = setInterval(go, 3000);
    return () => clearInterval(handle);
  }, [url]);

  return <>{children}</>;
}
